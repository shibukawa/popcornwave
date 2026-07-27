package pw

import (
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/shibukawa/tinybind-go/configbind"
)

// Boot log formats. Startup facts are collected during configuration parsing
// and emitted once, when the framework knows whether it owns a listener.
const (
	// BootLogAuto renders the tree on an interactive terminal and the single
	// structured record everywhere else.
	BootLogAuto = "auto"
	// BootLogTree writes the human-readable startup summary to stderr.
	BootLogTree = "tree"
	// BootLogRecord emits one structured log record through the default logger.
	BootLogRecord = "record"
	// BootLogOff suppresses the startup summary.
	BootLogOff = "off"
)

// bootMessage is the stable message of the structured startup record.
const bootMessage = "popcornwave started"

const redactedValue = "[REDACTED]"

type bootEntry struct {
	key    string
	value  string
	source string
}

// bootReport is everything the framework knows about this process at startup.
type bootReport struct {
	startedAt   time.Time
	environment string
	configPath  string
	configFound bool
	entries     []bootEntry
}

var bootState struct {
	sync.Mutex
	report  bootReport
	emitted bool
}

// captureBootReport records resolved configuration instead of logging one
// record per key. The summary is emitted later, by Run or Middlewares.
func captureBootReport(result *configbind.LoadResult) {
	report := bootReport{startedAt: time.Now(), environment: Env()}
	if result != nil {
		report.configPath, report.configFound = result.ConfigPath, result.FoundFile
		report.entries = bootEntries(result.Overlay)
	}
	bootState.Lock()
	bootState.report, bootState.emitted = report, false
	bootState.Unlock()
}

func bootEntries(overlay *configbind.Overlay) []bootEntry {
	if overlay == nil {
		return nil
	}
	keys := overlay.Keys()
	sort.Strings(keys)
	entries := make([]bootEntry, 0, len(keys))
	for _, key := range keys {
		entry, ok := overlay.Get(key)
		if !ok {
			continue
		}
		value := entry.Raw
		if isSecretKey(key) {
			value = redactedValue
		}
		entries = append(entries, bootEntry{key: key, value: value, source: string(entry.Place)})
	}
	return entries
}

// emitBootReport writes the startup summary exactly once. listening is the URL
// of the framework-owned listener, or empty when the application serves itself.
func emitBootReport(listening string) {
	bootState.Lock()
	defer bootState.Unlock()
	if bootState.emitted {
		return
	}
	bootState.emitted = true
	report := bootState.report
	switch resolveBootLogFormat(Config[ObservabilityConfig](nil).BootLog) {
	case BootLogOff:
	case BootLogTree:
		_, _ = os.Stderr.WriteString(renderBootTree(report, listening, bootStyleFor(os.Stderr)))
	default:
		slog.Info(bootMessage, bootRecordAttrs(report, listening)...)
	}
}

// resolveBootLogFormat maps the configured value to a concrete format. The
// terminal check is what separates a developer reading a summary from an
// operator collecting one structured event.
func resolveBootLogFormat(setting string) string {
	switch strings.ToLower(strings.TrimSpace(setting)) {
	case BootLogTree:
		return BootLogTree
	case BootLogRecord:
		return BootLogRecord
	case BootLogOff:
		return BootLogOff
	}
	if isTerminal(os.Stderr) {
		return BootLogTree
	}
	return BootLogRecord
}

func isTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// bootNode is one level of the dotted configuration key space.
type bootNode struct {
	name     string
	entry    *bootEntry
	children []*bootNode
	index    map[string]*bootNode
}

func (node *bootNode) child(name string) *bootNode {
	if existing, ok := node.index[name]; ok {
		return existing
	}
	created := &bootNode{name: name, index: map[string]*bootNode{}}
	node.index[name] = created
	node.children = append(node.children, created)
	return created
}

func buildBootTree(entries []bootEntry) *bootNode {
	root := &bootNode{index: map[string]*bootNode{}}
	for index := range entries {
		node := root
		for _, part := range strings.Split(entries[index].key, ".") {
			node = node.child(part)
		}
		node.entry = &entries[index]
	}
	return root
}

type bootLine struct {
	label string
	// column is where the value starts, so that keys sharing a parent line up
	// with each other instead of with the deepest key in the whole tree.
	column int
	// valueWidth is the widest value among those siblings, so their source
	// marks form a column of their own.
	valueWidth int
	entry      *bootEntry
}

func bootLines(node *bootNode, prefix string, lines []bootLine) []bootLine {
	column, valueWidth := 0, 0
	for _, child := range node.children {
		if child.entry != nil {
			column = max(column, utf8.RuneCountInString(prefix+"├─ "+child.name)+2)
			valueWidth = max(valueWidth, utf8.RuneCountInString(bootDisplayValue(child.entry.value)))
		}
	}
	for index, child := range node.children {
		branch, indent := "├─ ", prefix+"│  "
		if index == len(node.children)-1 {
			branch, indent = "└─ ", prefix+"   "
		}
		lines = append(lines, bootLine{
			label: prefix + branch + child.name, column: column, valueWidth: valueWidth, entry: child.entry,
		})
		lines = bootLines(child, indent, lines)
	}
	return lines
}

// renderBootTree formats the whole startup summary: banner, configuration
// grouped by section, and the address the server accepted.
func renderBootTree(report bootReport, listening string, style bootStyle) string {
	var out strings.Builder
	for _, line := range bootBanner(report, style) {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	lines := bootLines(buildBootTree(report.entries), "", nil)
	if len(lines) > 0 {
		out.WriteString(style.dim("configuration") + "\n")
	}
	for _, line := range lines {
		out.WriteString(line.label)
		if line.entry != nil {
			value := bootDisplayValue(line.entry.value)
			out.WriteString(strings.Repeat(" ", line.column-utf8.RuneCountInString(line.label)))
			out.WriteString(value)
			if source := bootSourceTag(line.entry.source); source != "" {
				out.WriteString(strings.Repeat(" ", line.valueWidth-utf8.RuneCountInString(value)))
				out.WriteString(style.dim("  ← " + source))
			}
		}
		out.WriteByte('\n')
	}
	if listening != "" {
		out.WriteString("\nlistening on " + style.bold(listening) + "\n")
	}
	return out.String()
}

// bootBannerArt is the framework's popcorn mascot. Every row is padded to the
// same width so the summary text lines up beside it, and every glyph is ASCII
// so no terminal renders it at an unexpected width.
var bootBannerArt = []string{
	"   .-.   .-.   ",
	" .(   ) (   ). ",
	"(   o     o   )",
	"(    \\___/    )",
	" '-.__.___.__-'",
}

func bootBanner(report bootReport, style bootStyle) []string {
	name := "Popcorn Wave"
	if version := frameworkVersion(); version != "" {
		name += " " + version
	}
	captions := []string{
		"",
		style.bold(name),
		"started at " + report.startedAt.Format("2006-01-02 15:04:05 MST"),
		"env " + report.environment + " · " + bootConfigCaption(report),
		"",
	}
	banner := make([]string, 0, len(bootBannerArt)+1)
	for index, art := range bootBannerArt {
		line := style.accent(art)
		if captions[index] != "" {
			line += "   " + captions[index]
		}
		banner = append(banner, line)
	}
	return append(banner, "")
}

// bootDisplayValue keeps an unset value visible instead of leaving the column
// blank, which reads as a rendering bug.
func bootDisplayValue(value string) string {
	if value == "" {
		return `""`
	}
	return value
}

func bootConfigCaption(report bootReport) string {
	if report.configFound && report.configPath != "" {
		return report.configPath
	}
	return "no config file (defaults, env, and flags only)"
}

// bootSourceTag names the layer that won a key. Defaults stay unmarked so the
// values an operator actually chose are the ones that catch the eye.
func bootSourceTag(source string) string {
	switch configbind.Place(source) {
	case configbind.PlaceDefault, "":
		return ""
	case configbind.PlaceFile:
		return "file"
	case configbind.PlaceCLI:
		return "flag"
	default:
		return source
	}
}

// bootRecordAttrs renders the same facts as one structured record: nested
// groups for values, a flat group naming every non-default source.
func bootRecordAttrs(report bootReport, listening string) []any {
	attrs := []any{
		slog.String("environment", report.environment),
		slog.Time("started_at", report.startedAt),
	}
	if version := frameworkVersion(); version != "" {
		attrs = append(attrs, slog.String("version", version))
	}
	if report.configFound && report.configPath != "" {
		attrs = append(attrs, slog.String("config_file", report.configPath))
	}
	if listening != "" {
		attrs = append(attrs, slog.String("listening", listening))
	}
	attrs = append(attrs, slog.Group("config", bootConfigAttrs(buildBootTree(report.entries))...))
	sources := make([]any, 0, len(report.entries))
	for _, entry := range report.entries {
		if tag := bootSourceTag(entry.source); tag != "" {
			sources = append(sources, slog.String(entry.key, tag))
		}
	}
	if len(sources) > 0 {
		attrs = append(attrs, slog.Group("config_source", sources...))
	}
	return attrs
}

func bootConfigAttrs(node *bootNode) []any {
	attrs := make([]any, 0, len(node.children))
	for _, child := range node.children {
		if child.entry != nil {
			attrs = append(attrs, slog.String(child.name, child.entry.value))
			continue
		}
		attrs = append(attrs, slog.Group(child.name, bootConfigAttrs(child)...))
	}
	return attrs
}

// bootStyle applies ANSI attributes only when the destination is a terminal
// that has not asked for plain output.
type bootStyle struct{ color bool }

func bootStyleFor(file *os.File) bootStyle {
	return bootStyle{color: isTerminal(file) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"}
}

func (style bootStyle) wrap(code, text string) string {
	if !style.color || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (style bootStyle) bold(text string) string   { return style.wrap("1", text) }
func (style bootStyle) dim(text string) string    { return style.wrap("2", text) }
func (style bootStyle) accent(text string) string { return style.wrap("33", text) }
