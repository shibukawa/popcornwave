// Package bootblock recognizes the startup summary an application printed and
// reports the next one as a difference from it.
//
// The two halves of a reload know different things. The application resolved the
// configuration, and is the only place the values and their winning places
// exist; api:cli-dev restarted the process, and is the only place that knows a
// restart happened. Rather than teach the application that reloads exist, the
// loop reads back what it printed.
//
// The cost of that is here and nowhere else: the layout of a startup summary
// becomes something a program reads as well as a person. The scanner and the
// parser below are the only code that assumes anything about it, and a block
// they cannot read is passed through unchanged, which is the behavior it
// replaces.
package bootblock

import (
	"strings"
	"unicode/utf8"

	"github.com/shibukawa/popcornwave/internal/pwtree"
)

// Art is the framework's popcorn mascot, and its first row is how a startup
// summary is found in a stream. Every row is padded to the same width so the
// summary text lines up beside it, and every glyph is ASCII so no terminal
// renders it at an unexpected width.
var Art = []string{
	"   .-.   .-.   ",
	" .(   ) (   ). ",
	"(   o     o   )",
	"(    \\___/    )",
	" '-.__.___.__-'",
}

// bannerLines is the art plus the blank line closing it. The captions written
// beside the art are free text, so counting is the only reliable way past them.
const bannerLines = 6

// configurationHeading introduces the tree, and listeningPrefix ends a summary
// emitted by Run. A summary emitted by Middlewares has no listening line, which
// is why the scanner also ends a block on the first line that cannot belong to
// one.
const (
	configurationHeading = "configuration"
	listeningPrefix      = "listening on "
	environmentPrefix    = "env "
	captionSeparator     = " · "
	// sourceMark is what pwtree writes between a value and the place that won
	// it. Two spaces are part of it: a tree pads its value column with at least
	// that many, so the mark cannot be confused with anything inside a value.
	sourceMark = "  ← "
)

// Scanner accumulates the lines of one startup summary as they arrive.
type Scanner struct{ lines []string }

// Feed offers one line. taken reports that the line belonged to the summary and
// has been kept; complete reports that the summary ends here, with this line
// when it was taken and before it when it was not.
func (s *Scanner) Feed(line string) (taken, complete bool) {
	text := stripANSI(line)
	if len(s.lines) == 0 {
		// Trailing whitespace is not depended on. The first row of the art ends
		// in spaces, and nothing between here and the application promises to
		// carry them.
		if strings.TrimRight(text, " ") != strings.TrimRight(Art[0], " ") {
			return false, false
		}
		s.lines = append(s.lines, line)
		return true, false
	}
	if len(s.lines) < bannerLines {
		s.lines = append(s.lines, line)
		return true, false
	}
	if strings.HasPrefix(text, listeningPrefix) {
		s.lines = append(s.lines, line)
		return true, true
	}
	if strings.TrimSpace(text) == "" || text == configurationHeading || isRow(text) {
		s.lines = append(s.lines, line)
		return true, false
	}
	return false, true
}

// Holding reports whether a summary is being accumulated.
func (s *Scanner) Holding() bool { return len(s.lines) > 0 }

// Take returns the accumulated lines and readies the scanner for the next
// summary.
func (s *Scanner) Take() []string {
	lines := s.lines
	s.lines = nil
	return lines
}

// Report is one startup summary read back from what it printed. Lines is kept
// so a caller can print the summary rather than describe it, which is what the
// first one of a session gets.
type Report struct {
	Lines       []string
	Version     string
	Environment string
	ConfigFile  string
	Listening   string
	Entries     []pwtree.Entry
}

// Parse reads a block the Scanner accumulated. It fails rather than guesses: a
// banner that no longer says what it used to leaves the caller with a block it
// should print unchanged.
func Parse(lines []string) (Report, bool) {
	if len(lines) < bannerLines {
		return Report{}, false
	}
	report := Report{Lines: lines, Version: caption(lines, 1)}
	environment, configFile, ok := splitEnvironmentCaption(caption(lines, 3))
	if !ok || report.Version == "" {
		return Report{}, false
	}
	report.Environment, report.ConfigFile = environment, configFile
	var path []string
	for _, line := range lines[bannerLines:] {
		text := stripANSI(line)
		if after, found := strings.CutPrefix(text, listeningPrefix); found {
			report.Listening = strings.TrimSpace(after)
			continue
		}
		depth, name, rest := splitLabel(text)
		if name == "" {
			continue
		}
		path = append(path[:min(depth, len(path))], name)
		if rest == "" {
			continue
		}
		value, source := splitValue(rest)
		report.Entries = append(report.Entries, pwtree.Entry{Key: strings.Join(path, "."), Value: value, Source: source})
	}
	return report, true
}

// Kind is what happened to one key between two summaries.
type Kind int

const (
	// Changed covers a new value and a new winning place alike, because the
	// summary reports the place for the same reason it reports the value.
	Changed Kind = iota
	// Added is a key the previous summary did not report at all, which is what
	// enabling a gated section looks like from here.
	Added
	// Removed is a key this summary no longer reports.
	Removed
)

// Change is one row of a reload report.
type Change struct {
	Key  string
	Kind Kind
	Old  pwtree.Entry
	New  pwtree.Entry
}

// FactChange is one thing the banner or the listening line said differently.
// These are not configuration keys, so they are reported above the tree rather
// than inside it.
type FactChange struct{ Label, Old, New string }

// Diff reports what the current summary says that the previous one did not.
// Removals come first and the rest follows the current summary's own order, so
// a section reads the way it reads in a full tree.
func Diff(previous, current Report) []Change {
	before, after := index(previous.Entries), index(current.Entries)
	changes := []Change{}
	for _, entry := range previous.Entries {
		if _, ok := after[entry.Key]; !ok {
			changes = append(changes, Change{Key: entry.Key, Kind: Removed, Old: entry})
		}
	}
	for _, entry := range current.Entries {
		old, ok := before[entry.Key]
		switch {
		case !ok:
			changes = append(changes, Change{Key: entry.Key, Kind: Added, New: entry})
		case old.Value != entry.Value || old.Source != entry.Source:
			changes = append(changes, Change{Key: entry.Key, Kind: Changed, Old: old, New: entry})
		}
	}
	return changes
}

// Facts compares what the banner and the listening line reported. The start
// time is not among them: comparing it would make every reload a change.
func Facts(previous, current Report) []FactChange {
	pairs := []FactChange{
		{Label: "version", Old: previous.Version, New: current.Version},
		{Label: "env", Old: previous.Environment, New: current.Environment},
		{Label: "config", Old: previous.ConfigFile, New: current.ConfigFile},
		{Label: "listening", Old: previous.Listening, New: current.Listening},
	}
	changed := []FactChange{}
	for _, pair := range pairs {
		if pair.Old != pair.New {
			changed = append(changed, pair)
		}
	}
	return changed
}

// Render writes a reload report: the facts that changed, then the changed rows
// in the tree they came from. dim styles the mark naming what happened to a row;
// pass a function that returns its argument unchanged to render without color.
func Render(out *strings.Builder, facts []FactChange, changes []Change, dim func(string) string) {
	width := 0
	for _, fact := range facts {
		width = max(width, utf8.RuneCountInString(fact.Label))
	}
	for _, fact := range facts {
		out.WriteString(fact.Label)
		out.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(fact.Label)+3))
		out.WriteString(arrow(fact.Old, fact.New))
		out.WriteByte('\n')
	}
	entries := make([]pwtree.Entry, 0, len(changes))
	for _, change := range changes {
		entries = append(entries, pwtree.Entry{Key: change.Key, Value: change.value(), Source: change.mark()})
	}
	pwtree.Render(out, pwtree.Lines(entries), func(mark string) string { return mark }, dim)
}

// value is what the row shows in the value column. A key whose value survived
// and whose winning place did not shows the value once: the change is in the
// mark, and printing the same text twice with an arrow between reads as a
// rendering fault.
func (c Change) value() string {
	switch {
	case c.Kind == Added:
		return pwtree.DisplayValue(c.New.Value)
	case c.Kind == Removed:
		return pwtree.DisplayValue(c.Old.Value)
	case c.Old.Value == c.New.Value:
		return pwtree.DisplayValue(c.New.Value)
	}
	return arrow(c.Old.Value, c.New.Value)
}

// mark names what happened to the row, in the column a full tree uses for the
// place that won a key.
func (c Change) mark() string {
	switch {
	case c.Kind == Added:
		return "added"
	case c.Kind == Removed:
		return "removed"
	case c.Old.Source != c.New.Source:
		return arrow(place(c.Old.Source), place(c.New.Source))
	}
	return c.New.Source
}

// place names the layer a key came from. A tree leaves a default unmarked, but
// a row saying a key stopped being a default has to name what it was.
func place(source string) string {
	if source == "" {
		return "default"
	}
	return source
}

func arrow(old, current string) string {
	return pwtree.DisplayValue(old) + " → " + pwtree.DisplayValue(current)
}

func index(entries []pwtree.Entry) map[string]pwtree.Entry {
	byKey := make(map[string]pwtree.Entry, len(entries))
	for _, entry := range entries {
		byKey[entry.Key] = entry
	}
	return byKey
}

// caption returns the text written beside one row of the art. It cuts at the
// width every row shares rather than at the row itself, so a caption is read the
// same way whether or not the art before it kept its trailing spaces.
func caption(lines []string, index int) string {
	runes := []rune(stripANSI(lines[index]))
	width := utf8.RuneCountInString(Art[index])
	if len(runes) <= width {
		return ""
	}
	return strings.TrimSpace(string(runes[width:]))
}

func splitEnvironmentCaption(text string) (environment, configFile string, ok bool) {
	rest, found := strings.CutPrefix(text, environmentPrefix)
	if !found {
		return "", "", false
	}
	environment, configFile, found = strings.Cut(rest, captionSeparator)
	if !found {
		return "", "", false
	}
	return environment, configFile, true
}

// isRow reports whether a line is a row of the tree rather than something
// printed around it.
func isRow(text string) bool {
	_, name, _ := splitLabel(text)
	return name != ""
}

// splitLabel reads a row's branch characters back into a depth and a name, and
// returns whatever followed the column padding. A name is taken up to the first
// run of two spaces, which is safe in the one direction that matters: a tree
// label is a dotted key segment and a pad, so two spaces inside one cannot
// happen, while a value containing them is only ever on the other side of the
// cut.
func splitLabel(text string) (depth int, name, rest string) {
	runes := []rune(text)
	position := 0
	for position+3 <= len(runes) {
		switch string(runes[position : position+3]) {
		case "│  ", "   ":
			position, depth = position+3, depth+1
			continue
		}
		break
	}
	if position+3 > len(runes) {
		return 0, "", ""
	}
	switch string(runes[position : position+3]) {
	case "├─ ", "└─ ":
		position += 3
	default:
		return 0, "", ""
	}
	rest = string(runes[position:])
	cut := strings.Index(rest, "  ")
	if cut < 0 {
		return depth, strings.TrimRight(rest, " "), ""
	}
	return depth, rest[:cut], strings.TrimLeft(rest[cut:], " ")
}

// splitValue separates a value from the place that won it. The mark is looked
// for from the right, so a value that happens to contain one keeps it.
func splitValue(rest string) (value, source string) {
	cut := strings.LastIndex(rest, sourceMark)
	if cut < 0 {
		return strings.TrimRight(rest, " "), ""
	}
	return strings.TrimRight(rest[:cut], " "), strings.TrimSpace(rest[cut+len(sourceMark):])
}

// stripANSI removes the styling a terminal was meant to interpret, so what is
// compared is what was said rather than how it was drawn.
func stripANSI(text string) string {
	if !strings.Contains(text, "\x1b[") {
		return text
	}
	var out strings.Builder
	for position := 0; position < len(text); {
		if text[position] != '\x1b' || position+1 >= len(text) || text[position+1] != '[' {
			out.WriteByte(text[position])
			position++
			continue
		}
		end := position + 2
		for end < len(text) && text[end] != 'm' {
			end++
		}
		position = end + 1
	}
	return out.String()
}
