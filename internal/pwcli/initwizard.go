package pwcli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errInitCanceled reports that the operator dismissed the wizard.
var errInitCanceled = errors.New("canceled")

// wizardStep is one question of the pw init wizard. A new project option needs
// one field on initOptions, one shortcut flag in parseInitArgs, one step in
// initWizardSteps, and whatever scaffoldFiles has to emit for it; the wizard
// itself stays untouched.
type wizardStep interface {
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
	apply(target *initOptions)
}

// conditionalStep is a step that only applies to some answer combinations. A
// step that reports false is skipped, left off the review screen, and never
// applied, so a follow-up question cannot leak an answer into a project that
// did not ask it.
type conditionalStep interface {
	applies(options initOptions) bool
}

// whenStep attaches a condition to any step.
type whenStep struct {
	wizardStep
	condition func(initOptions) bool
}

func when(condition func(initOptions) bool, step wizardStep) wizardStep {
	return &whenStep{wizardStep: step, condition: condition}
}

func (s *whenStep) applies(options initOptions) bool { return s.condition(options) }

// initWizardSteps builds the question list, seeding every answer from the
// shortcut flags that were already supplied on the command line.
func initWizardSteps(defaults initOptions) []wizardStep {
	return []wizardStep{
		newTextStep(
			"Project name",
			"Creates ./<name> holding a Go module of the same name.",
			defaults.Name,
			"myapp",
			validateProjectName,
			func(target *initOptions, value string) { target.Name = value },
		),
		newChoiceStep(
			"TinyGo support",
			"TinyGo produces much smaller binaries and has the more complete wasm target.",
			yesNoCursor(defaults.TinyGo),
			wizardChoice{
				name:        "Yes",
				description: "pw.ServeMux routing plus the TinyGo toolchain in devbox.json",
				apply:       func(target *initOptions) { target.TinyGo = true },
			},
			wizardChoice{
				name:        "No",
				description: "net/http.ServeMux routing, host Go toolchain only",
				apply:       func(target *initOptions) { target.TinyGo = false },
			},
		),
		newChoiceStep(
			"Tailwind CSS",
			"Wires the pinned Tailwind toolchain into the project and generates public/generated/app.css.",
			yesNoCursor(defaults.Tailwind),
			wizardChoice{
				name:        "Yes",
				description: "assets/app.css entry point and the Tailwind build step",
				apply:       func(target *initOptions) { target.Tailwind = true },
			},
			wizardChoice{
				name:        "No",
				description: "plain CSS owned by the application",
				apply:       func(target *initOptions) { target.Tailwind = false },
			},
		),
		newChoiceStep(
			"Authentication",
			"Selects the login model. The framework writes the [auth] configuration; handlers stay yours.",
			authCursor(defaults.Auth),
			wizardChoice{
				name:        "None",
				description: "no authentication configuration",
				apply:       setAuth(authNone),
			},
			wizardChoice{
				name:        "OIDC",
				description: "log in against an OpenID Provider",
				apply:       setAuth(authOIDC),
			},
			wizardChoice{
				name:        "Passkey only",
				description: "not implemented yet; the choice is recorded, disabled",
				apply:       setAuth(authPasskey),
			},
		),
		when(func(options initOptions) bool { return usesOIDC(options.Auth) },
			newChoiceStep(
				"OIDC provider",
				"The local emulator signs you in by picking a user from a list, so login works before a real IdP exists.",
				yesNoCursor(defaults.AuthEmulator),
				wizardChoice{
					name:        "Local emulator",
					description: "pw dev runs it and injects the issuer and client credentials",
					apply:       func(target *initOptions) { target.AuthEmulator = true },
				},
				wizardChoice{
					name:        "External provider",
					description: "fill in auth.oidc yourself, or supply it through the environment",
					apply:       func(target *initOptions) { target.AuthEmulator = false },
				},
			),
		),
	}
}

// setAuth records the mode and clears the emulator answer for a mode that has
// no provider, so a stray --devidp flag cannot survive the choice.
func setAuth(mode string) func(*initOptions) {
	return func(target *initOptions) {
		target.Auth = mode
		if !usesOIDC(mode) {
			target.AuthEmulator = false
		}
	}
}

// authCursor maps an authentication mode onto its position in the choice list.
func authCursor(mode string) int {
	switch mode {
	case authOIDC:
		return 1
	case authPasskey:
		return 2
	default:
		return 0
	}
}

// yesNoCursor maps a boolean default onto a leading yes, trailing no choice list.
func yesNoCursor(enabled bool) int {
	if enabled {
		return 0
	}
	return 1
}

// runInitWizard asks every question and returns the confirmed options. The
// program options exist so tests can drive the wizard without a terminal.
func runInitWizard(defaults initOptions, programOptions ...tea.ProgramOption) (initOptions, error) {
	model := wizardModel{steps: initWizardSteps(defaults), defaults: defaults, theme: newWizardTheme()}
	program := tea.NewProgram(model, programOptions...)
	final, err := program.Run()
	if err != nil {
		return initOptions{}, err
	}
	completed, ok := final.(wizardModel)
	if !ok || !completed.confirmed {
		return initOptions{}, errInitCanceled
	}
	return completed.answers(), nil
}

type wizardModel struct {
	steps     []wizardStep
	defaults  initOptions
	index     int
	confirmed bool
	canceled  bool
	theme     wizardTheme
}

// answers folds every applicable step into the collected options. A
// conditional step is evaluated against the answers that precede it, which is
// why the fold runs in step order.
func (m wizardModel) answers() initOptions {
	options := m.defaults
	for _, step := range m.steps {
		if conditional, ok := step.(conditionalStep); ok && !conditional.applies(options) {
			continue
		}
		step.apply(&options)
	}
	return options
}

// activeSteps lists the indexes of the steps the current answers ask for.
func (m wizardModel) activeSteps() []int {
	options := m.defaults
	var active []int
	for index, step := range m.steps {
		if conditional, ok := step.(conditionalStep); ok && !conditional.applies(options) {
			continue
		}
		step.apply(&options)
		active = append(active, index)
	}
	return active
}

// step moves from the current index to the next or previous active step, or
// past the end when the wizard is done.
func (m wizardModel) step(delta int) int {
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

func (m wizardModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return nil
	}
	return m.steps[0].focus()
}

// reviewing reports whether the review screen rather than a step is current.
func (m wizardModel) reviewing() bool { return m.index >= len(m.steps) }

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		if m.reviewing() {
			return m, nil
		}
		cmd, _ := m.steps[m.index].update(msg)
		return m, cmd
	}
	switch key.Type {
	case tea.KeyCtrlC:
		m.canceled = true
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyShiftTab:
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
	m.index = m.step(1)
	if m.reviewing() {
		return m, cmd
	}
	return m, tea.Batch(cmd, m.steps[m.index].focus())
}

func (m wizardModel) View() string {
	// Leave the terminal clean; the CLI reports the outcome itself.
	if m.confirmed || m.canceled {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n  " + m.theme.title.Render("Popcorn Wave  new project") + "\n\n")
	if m.reviewing() {
		out.WriteString("  " + m.theme.step.Render("Review") + "\n\n")
		out.WriteString(m.reviewRows())
		out.WriteString("\n  " + m.theme.footer.Render("enter create  ·  esc back  ·  ctrl+c cancel") + "\n")
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

func (m wizardModel) footerHint() string {
	hint := "enter next"
	if _, ok := unwrapStep(m.steps[m.index]).(*choiceStep); ok {
		hint = "↑/↓ move  ·  " + hint
	}
	if m.step(-1) >= 0 {
		hint += "  ·  esc back"
	}
	return hint + "  ·  ctrl+c cancel"
}

// unwrapStep returns the step a condition wraps, so the footer can still tell
// which keys the current question accepts.
func unwrapStep(step wizardStep) wizardStep {
	if wrapped, ok := step.(*whenStep); ok {
		return unwrapStep(wrapped.wizardStep)
	}
	return step
}

func (m wizardModel) reviewRows() string {
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

type wizardChoice struct {
	name        string
	description string
	apply       func(*initOptions)
}

type choiceStep struct {
	name    string
	reason  string
	choices []wizardChoice
	cursor  int
}

func newChoiceStep(name, reason string, cursor int, choices ...wizardChoice) *choiceStep {
	if cursor < 0 || cursor >= len(choices) {
		cursor = 0
	}
	return &choiceStep{name: name, reason: reason, choices: choices, cursor: cursor}
}

func (s *choiceStep) label() string   { return s.name }
func (s *choiceStep) explain() string { return s.reason }
func (s *choiceStep) focus() tea.Cmd  { return nil }
func (s *choiceStep) value() string   { return s.choices[s.cursor].name }

func (s *choiceStep) apply(target *initOptions) { s.choices[s.cursor].apply(target) }

func (s *choiceStep) update(msg tea.Msg) (tea.Cmd, bool) {
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

func (s *choiceStep) move(delta int) {
	s.cursor = (s.cursor + delta + len(s.choices)) % len(s.choices)
}

func (s *choiceStep) view(theme wizardTheme) string {
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

type textStep struct {
	name     string
	reason   string
	input    textinput.Model
	validate func(string) error
	assign   func(*initOptions, string)
	failure  string
}

func newTextStep(name, reason, initial, placeholder string, validate func(string) error, assign func(*initOptions, string)) *textStep {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = ""
	input.CharLimit = 64
	input.Width = 40
	input.SetValue(initial)
	return &textStep{name: name, reason: reason, input: input, validate: validate, assign: assign}
}

func (s *textStep) label() string   { return s.name }
func (s *textStep) explain() string { return s.reason }
func (s *textStep) value() string   { return strings.TrimSpace(s.input.Value()) }

func (s *textStep) focus() tea.Cmd { return s.input.Focus() }

func (s *textStep) apply(target *initOptions) { s.assign(target, s.value()) }

func (s *textStep) update(msg tea.Msg) (tea.Cmd, bool) {
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

func (s *textStep) view(theme wizardTheme) string {
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
