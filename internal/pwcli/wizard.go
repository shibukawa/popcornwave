package pwcli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// wizardStep is one question of a pw wizard. A new option costs one field on
// the answer type, one step in the command's step list, and whatever the
// command emits for it; the wizard itself stays untouched.
type wizardStep[T any] interface {
	// label names the step on both its own screen and the review screen.
	label() string
	// explain is the rationale shown under the label.
	explain() string
	// focus prepares the step for input when it becomes current.
	focus() tea.Cmd
	// update handles one message and reports whether the answer is accepted.
	update(msg tea.Msg) (tea.Cmd, bool)
	// view renders the interactive part of the step.
	view(theme wizardTheme) string
	// value is the accepted answer as shown on the review screen.
	value() string
	// apply writes the accepted answer into the collected options.
	apply(target *T)
}

// conditionalStep is a step that only applies to some answer combinations. A
// step that reports false is skipped, left off the review screen, and never
// applied, so a follow-up question cannot leak an answer into a project that
// did not ask it.
type conditionalStep[T any] interface {
	applies(options T) bool
}

// whenStep attaches a condition to any step.
type whenStep[T any] struct {
	wizardStep[T]
	condition func(T) bool
}

func when[T any](condition func(T) bool, step wizardStep[T]) wizardStep[T] {
	return &whenStep[T]{wizardStep: step, condition: condition}
}

// applies reports whether this step and every condition wrapped inside it
// hold. Conditions nest: a question can be one a project kind never asks and
// also one a set of answers never reaches, and the outer wrapper must not
// answer for the inner one.
func (s *whenStep[T]) applies(options T) bool {
	if !s.condition(options) {
		return false
	}
	inner, ok := s.wizardStep.(conditionalStep[T])
	return !ok || inner.applies(options)
}

// errWizardCanceled reports that the operator dismissed the wizard.
var errWizardCanceled = errors.New("canceled")

// runWizard asks every question and returns the confirmed answers. The program
// options exist so tests can drive a wizard without a terminal.
func runWizard[T any](model wizardModel[T], programOptions ...tea.ProgramOption) (T, error) {
	var zero T
	program := tea.NewProgram(model, programOptions...)
	final, err := program.Run()
	if err != nil {
		return zero, err
	}
	completed, ok := final.(wizardModel[T])
	if !ok || !completed.confirmed {
		return zero, errWizardCanceled
	}
	return completed.answers(), nil
}

type wizardModel[T any] struct {
	steps    []wizardStep[T]
	defaults T
	// title names the wizard, and confirm names what accepting the review does.
	title   string
	confirm string
	// plan renders what the answers will do to the project. init only has to
	// show the answers; a command that edits an existing project shows the
	// files it would write, because that is what the operator approves.
	plan      func(T) []string
	index     int
	confirmed bool
	canceled  bool
	theme     wizardTheme

	// linear is how many leading steps are asked one after another before the
	// answer hub takes over. Zero keeps the whole wizard linear, which is what
	// a command reviewing a file plan wants: it approves an effect on a project
	// that already exists, and its review is a list to read rather than edit.
	linear int
	// rebuild reseeds the steps from the answers so far. init uses it for the
	// preset step, because choosing a preset is choosing every answer below it
	// and a hub row has to show that answer rather than the default it would
	// otherwise have carried.
	rebuild func(T) []wizardStep[T]
	// inHub reports that the answer list is current rather than a step.
	inHub bool
	// editing is the step open over the hub, or hubNoStep when the hub itself
	// holds the cursor.
	editing int
	// cursor is the step index the hub has selected, or hubConfirmRow.
	cursor int
	// answered records which steps hold a value somebody chose, so a row can
	// say whether it is a decision or a default nobody looked at.
	answered map[int]bool
}

// Hub sentinels. Both are step indexes that name no step: one for the confirm
// row under the answers, and one for "no step is open".
const (
	hubConfirmRow = -1
	hubNoStep     = -1
)

// answers folds every applicable step into the collected options. A
// conditional step is evaluated against the answers that precede it, which is
// why the fold runs in step order.
func (m wizardModel[T]) answers() T {
	options := m.defaults
	for _, step := range m.steps {
		if conditional, ok := step.(conditionalStep[T]); ok && !conditional.applies(options) {
			continue
		}
		step.apply(&options)
	}
	return options
}

// activeSteps lists the indexes of the steps the current answers ask for.
func (m wizardModel[T]) activeSteps() []int {
	options := m.defaults
	var active []int
	for index, step := range m.steps {
		if conditional, ok := step.(conditionalStep[T]); ok && !conditional.applies(options) {
			continue
		}
		step.apply(&options)
		active = append(active, index)
	}
	return active
}

// step moves from the current index to the next or previous active step, or
// past the end when the wizard is done.
func (m wizardModel[T]) step(delta int) int {
	active := m.activeSteps()
	if delta > 0 {
		for _, index := range active {
			if index > m.index {
				return index
			}
		}
		return len(m.steps)
	}
	previous := -1
	for _, index := range active {
		if index < m.index {
			previous = index
		}
	}
	return previous
}

func (m wizardModel[T]) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return nil
	}
	return m.steps[0].focus()
}

// reviewing reports whether the review screen rather than a step is current.
func (m wizardModel[T]) reviewing() bool { return m.index >= len(m.steps) }

func (m wizardModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.answered == nil {
		// Allocated here rather than in every constructor, because Update
		// returns the model and a wizard built by hand in a test is as valid
		// as one a command built.
		m.answered = map[int]bool{}
	}
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		if m.onAnswerList() || m.reviewing() {
			return m, nil
		}
		cmd, _ := m.steps[m.index].update(msg)
		return m, cmd
	}
	if key.Type == tea.KeyCtrlC {
		m.canceled = true
		return m, tea.Quit
	}
	if m.inHub {
		return m.updateHub(key, msg)
	}
	if key.Type == tea.KeyEsc || key.Type == tea.KeyShiftTab {
		// The footer only offers ctrl+c for cancelling, so esc never discards work.
		if previous := m.step(-1); previous >= 0 {
			m.index = previous
			return m, m.steps[m.index].focus()
		}
		return m, nil
	}
	if m.reviewing() {
		if key.Type == tea.KeyEnter {
			m.confirmed = true
			return m, tea.Quit
		}
		return m, nil
	}
	cmd, accepted := m.steps[m.index].update(msg)
	if !accepted {
		return m, cmd
	}
	m.answered[m.index] = true
	m = m.reseed(m.index)
	next := m.step(1)
	// The leading questions are walked; everything after them is a row on the
	// hub. Reaching that boundary is what opens the hub for the first time.
	// The count is of asked questions rather than of step indexes, because a
	// skipped step is not a question anybody was walked through.
	if m.linear > 0 && m.askedBefore(next) >= m.linear {
		return m, tea.Batch(cmd, m.openHub())
	}
	m.index = next
	if m.reviewing() {
		return m, cmd
	}
	return m, tea.Batch(cmd, m.steps[m.index].focus())
}

// updateHub handles a key while the answer list is current, or while a step
// opened from it is.
func (m wizardModel[T]) updateHub(key tea.KeyMsg, msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.editing != hubNoStep {
		if key.Type == tea.KeyEsc || key.Type == tea.KeyShiftTab {
			// Leaving a question returns to the list rather than to the
			// question before it: on a hub there is no question before it.
			m.editing = hubNoStep
			return m, nil
		}
		cmd, accepted := m.steps[m.editing].update(msg)
		if !accepted {
			return m, cmd
		}
		answered := m.editing
		m.answered[answered] = true
		m.cursor = answered
		m.editing = hubNoStep
		return m.reseed(answered), cmd
	}
	rows := m.activeSteps()
	switch key.Type {
	case tea.KeyEnter:
		if m.cursor == hubConfirmRow {
			m.confirmed = true
			return m, tea.Quit
		}
		m.editing = m.cursor
		m.index = m.cursor
		return m, m.steps[m.editing].focus()
	case tea.KeyUp:
		m.cursor = hubNeighbour(rows, m.cursor, -1)
		return m, nil
	case tea.KeyDown:
		m.cursor = hubNeighbour(rows, m.cursor, 1)
		return m, nil
	}
	switch key.String() {
	case "k":
		m.cursor = hubNeighbour(rows, m.cursor, -1)
	case "j":
		m.cursor = hubNeighbour(rows, m.cursor, 1)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if row := int(key.String()[0] - '1'); row < len(rows) {
			m.cursor = rows[row]
		}
	}
	return m, nil
}

// askedBefore counts the applicable steps ahead of an index, which is how many
// questions a walk reaching it has already asked.
func (m wizardModel[T]) askedBefore(index int) int {
	asked := 0
	for _, active := range m.activeSteps() {
		if active >= index {
			break
		}
		asked++
	}
	return asked
}

// openHub switches from the linear walk to the answer list, selecting the
// first row a reader would edit rather than the confirm row.
func (m *wizardModel[T]) openHub() tea.Cmd {
	m.inHub = true
	m.editing = hubNoStep
	rows := m.activeSteps()
	// The leading questions were just walked, so the cursor starts on the
	// first answer the walk did not ask about. With none, the list is a review
	// and the confirm row is where the cursor belongs.
	m.cursor = hubConfirmRow
	if len(rows) > m.linear {
		m.cursor = rows[m.linear]
	}
	return nil
}

// hubNeighbour moves the cursor over the answer rows and the confirm row
// beneath them, keeping it on a row that exists.
func hubNeighbour(rows []int, cursor, delta int) int {
	// The confirm row sits after the answers, so the list to move over is the
	// rows plus one.
	ordered := append(append([]int{}, rows...), hubConfirmRow)
	position := len(ordered) - 1
	for order, index := range ordered {
		if index == cursor {
			position = order
			break
		}
	}
	position = (position + delta + len(ordered)) % len(ordered)
	return ordered[position]
}

// reseed rebuilds the steps from the answers up to and including the one just
// accepted, so a question whose preselection depends on an earlier answer shows
// that answer rather than the default it was constructed with.
//
// The fold stops at that step rather than running to the end, and the
// difference is the whole point. A preset answers every question below it, and
// every one of those questions still carries the cursor it was built with. A
// full fold would let those stale cursors write over what the preset chose,
// on the way to deciding what to seed them with.
func (m wizardModel[T]) reseed(through int) wizardModel[T] {
	if m.rebuild == nil {
		return m
	}
	m.steps = m.rebuild(m.answersThrough(through))
	return m
}

// answersThrough folds the applicable steps up to and including one index.
func (m wizardModel[T]) answersThrough(through int) T {
	options := m.defaults
	for index, step := range m.steps {
		if index > through {
			break
		}
		if conditional, ok := step.(conditionalStep[T]); ok && !conditional.applies(options) {
			continue
		}
		step.apply(&options)
	}
	return options
}

// onAnswerList reports that the hub itself is current, with no step open over it.
func (m wizardModel[T]) onAnswerList() bool { return m.inHub && m.editing == hubNoStep }

func (m wizardModel[T]) View() string {
	// Leave the terminal clean; the CLI reports the outcome itself.
	if m.confirmed || m.canceled {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n  " + m.theme.title.Render(m.title) + "\n\n")
	if m.onAnswerList() {
		out.WriteString("  " + m.theme.step.Render("Review") + "\n")
		out.WriteString("  " + m.theme.explain.Render("enter opens a question; every answer here is still an answer") + "\n\n")
		out.WriteString(m.hubRows())
		if m.plan != nil {
			out.WriteString("\n")
			for _, line := range m.plan(m.answers()) {
				out.WriteString("    " + m.theme.explain.Render(line) + "\n")
			}
		}
		out.WriteString("\n  " + m.theme.footer.Render("↑/↓ move  ·  enter open  ·  ctrl+c cancel") + "\n")
		return out.String()
	}
	if m.inHub {
		current := m.steps[m.editing]
		out.WriteString("  " + m.theme.label.Render(current.label()) + "\n")
		out.WriteString("  " + m.theme.explain.Render(current.explain()) + "\n\n")
		out.WriteString(current.view(m.theme))
		hint := "enter accept  ·  esc back  ·  ctrl+c cancel"
		if _, ok := unwrapStep(current).(*choiceStep[T]); ok {
			hint = "↑/↓ move  ·  " + hint
		}
		out.WriteString("\n  " + m.theme.footer.Render(hint) + "\n")
		return out.String()
	}
	if m.reviewing() {
		out.WriteString("  " + m.theme.step.Render("Review") + "\n\n")
		out.WriteString(m.reviewRows())
		if m.plan != nil {
			out.WriteString("\n")
			for _, line := range m.plan(m.answers()) {
				out.WriteString("    " + m.theme.explain.Render(line) + "\n")
			}
		}
		out.WriteString("\n  " + m.theme.footer.Render("enter "+m.confirm+"  ·  esc back  ·  ctrl+c cancel") + "\n")
		return out.String()
	}
	current := m.steps[m.index]
	active := m.activeSteps()
	position := 1
	for order, index := range active {
		if index == m.index {
			position = order + 1
		}
	}
	out.WriteString("  " + m.theme.step.Render(fmt.Sprintf("Step %d/%d", position, len(active))) + "\n")
	out.WriteString("  " + m.theme.label.Render(current.label()) + "\n")
	out.WriteString("  " + m.theme.explain.Render(current.explain()) + "\n\n")
	out.WriteString(current.view(m.theme))
	out.WriteString("\n  " + m.theme.footer.Render(m.footerHint()) + "\n")
	return out.String()
}

func (m wizardModel[T]) footerHint() string {
	hint := "enter next"
	if _, ok := unwrapStep(m.steps[m.index]).(*choiceStep[T]); ok {
		hint = "↑/↓ move  ·  " + hint
	}
	if m.step(-1) >= 0 {
		hint += "  ·  esc back"
	}
	return hint + "  ·  ctrl+c cancel"
}

// unwrapStep returns the step a condition wraps, so the footer can still tell
// which keys the current question accepts.
func unwrapStep[T any](step wizardStep[T]) wizardStep[T] {
	if wrapped, ok := step.(*whenStep[T]); ok {
		return unwrapStep(wrapped.wizardStep)
	}
	return step
}

// hubRows renders the answer list: every applicable question with its current
// value, and the confirm row beneath them.
func (m wizardModel[T]) hubRows() string {
	active := m.activeSteps()
	width := 0
	for _, index := range active {
		width = max(width, lipgloss.Width(m.steps[index].label()))
	}
	var out strings.Builder
	for _, index := range active {
		step := m.steps[index]
		marker := "  "
		label := m.theme.explain
		if index == m.cursor {
			marker = m.theme.cursor.Render("❯ ")
			label = m.theme.label
		}
		out.WriteString("  " + marker + label.Render(fmt.Sprintf("%-*s", width, step.label())))
		out.WriteString("  " + m.theme.selected.Render(step.value()))
		// A row nobody opened is showing what it would have been asked with,
		// which on a list with no first-to-last order is the only way to say
		// which answers were considered.
		if !m.answered[index] {
			out.WriteString("  " + m.theme.explain.Render("(default)"))
		}
		out.WriteString("\n")
	}
	marker := "  "
	confirm := m.theme.explain
	if m.cursor == hubConfirmRow {
		marker = m.theme.cursor.Render("❯ ")
		confirm = m.theme.selected
	}
	out.WriteString("\n  " + marker + confirm.Render(m.confirm) + "\n")
	return out.String()
}

func (m wizardModel[T]) reviewRows() string {
	active := m.activeSteps()
	width := 0
	for _, index := range active {
		width = max(width, lipgloss.Width(m.steps[index].label()))
	}
	var out strings.Builder
	for _, index := range active {
		step := m.steps[index]
		out.WriteString("    " + m.theme.explain.Render(fmt.Sprintf("%-*s", width, step.label())))
		out.WriteString("  " + m.theme.selected.Render(step.value()) + "\n")
	}
	return out.String()
}

type wizardChoice[T any] struct {
	name        string
	description string
	apply       func(*T)
}

type choiceStep[T any] struct {
	name    string
	reason  string
	choices []wizardChoice[T]
	cursor  int
}

func newChoiceStep[T any](name, reason string, cursor int, choices ...wizardChoice[T]) *choiceStep[T] {
	if cursor < 0 || cursor >= len(choices) {
		cursor = 0
	}
	return &choiceStep[T]{name: name, reason: reason, choices: choices, cursor: cursor}
}

func (s *choiceStep[T]) label() string   { return s.name }
func (s *choiceStep[T]) explain() string { return s.reason }
func (s *choiceStep[T]) focus() tea.Cmd  { return nil }
func (s *choiceStep[T]) value() string   { return s.choices[s.cursor].name }

func (s *choiceStep[T]) apply(target *T) { s.choices[s.cursor].apply(target) }

func (s *choiceStep[T]) update(msg tea.Msg) (tea.Cmd, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	switch key.Type {
	case tea.KeyEnter, tea.KeyTab:
		return nil, true
	case tea.KeyUp:
		s.move(-1)
		return nil, false
	case tea.KeyDown:
		s.move(1)
		return nil, false
	}
	switch key.String() {
	case "k":
		s.move(-1)
	case "j":
		s.move(1)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Digits jump straight to a choice so the wizard stays keyboard-fast.
		if index := int(key.String()[0] - '1'); index < len(s.choices) {
			s.cursor = index
			return nil, true
		}
	}
	return nil, false
}

func (s *choiceStep[T]) move(delta int) {
	s.cursor = (s.cursor + delta + len(s.choices)) % len(s.choices)
}

func (s *choiceStep[T]) view(theme wizardTheme) string {
	var out strings.Builder
	for index, choice := range s.choices {
		if index == s.cursor {
			out.WriteString("  " + theme.cursor.Render("❯ ") + theme.selected.Render(choice.name) + "\n")
			out.WriteString("      " + theme.explain.Render(choice.description) + "\n")
			continue
		}
		out.WriteString("    " + theme.option.Render(choice.name) + "\n")
	}
	return out.String()
}

type textStep[T any] struct {
	name     string
	reason   string
	input    textinput.Model
	validate func(string) error
	assign   func(*T, string)
	failure  string
}

func newTextStep[T any](name, reason, initial, placeholder string, validate func(string) error, assign func(*T, string)) *textStep[T] {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = ""
	input.CharLimit = 64
	input.Width = 40
	input.SetValue(initial)
	return &textStep[T]{name: name, reason: reason, input: input, validate: validate, assign: assign}
}

func (s *textStep[T]) label() string   { return s.name }
func (s *textStep[T]) explain() string { return s.reason }
func (s *textStep[T]) value() string   { return strings.TrimSpace(s.input.Value()) }

func (s *textStep[T]) focus() tea.Cmd { return s.input.Focus() }

func (s *textStep[T]) apply(target *T) { s.assign(target, s.value()) }

func (s *textStep[T]) update(msg tea.Msg) (tea.Cmd, bool) {
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		value := s.value()
		if err := s.validate(value); err != nil {
			s.failure = err.Error()
			return nil, false
		}
		s.failure = ""
		s.input.SetValue(value)
		return nil, true
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	s.failure = ""
	return cmd, false
}

func (s *textStep[T]) view(theme wizardTheme) string {
	out := "  " + theme.cursor.Render("❯ ") + s.input.View() + "\n"
	if s.failure != "" {
		out += "    " + theme.failure.Render(s.failure) + "\n"
	}
	return out
}

type wizardTheme struct {
	title    lipgloss.Style
	step     lipgloss.Style
	label    lipgloss.Style
	explain  lipgloss.Style
	option   lipgloss.Style
	selected lipgloss.Style
	cursor   lipgloss.Style
	failure  lipgloss.Style
	footer   lipgloss.Style
}

func newWizardTheme() wizardTheme {
	// Both accents degrade to a readable ANSI yellow on 4-bit terminals.
	accent := lipgloss.AdaptiveColor{Light: "#8a4b00", Dark: "#ffc866"}
	muted := lipgloss.AdaptiveColor{Light: "#6c6c6c", Dark: "#9a9a9a"}
	return wizardTheme{
		title:    lipgloss.NewStyle().Bold(true).Foreground(accent),
		step:     lipgloss.NewStyle().Foreground(muted),
		label:    lipgloss.NewStyle().Bold(true),
		explain:  lipgloss.NewStyle().Foreground(muted),
		option:   lipgloss.NewStyle(),
		selected: lipgloss.NewStyle().Bold(true).Foreground(accent),
		cursor:   lipgloss.NewStyle().Foreground(accent),
		failure:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#a40000", Dark: "#ff6b6b"}),
		footer:   lipgloss.NewStyle().Foreground(muted),
	}
}
