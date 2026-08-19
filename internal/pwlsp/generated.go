package pwlsp

// pw/generatedFor: the Go a declaration produced.
//
// requirement:editor-generated-peek asks for it as a read-only view, because
// requirement:editor-navigation resolves through a *_pw_gen.go and never lands
// in it — which is right for navigation and leaves no way to answer what a
// declaration generated.
//
// Nothing is generated on demand. policy:editor-tool-execution keeps a write
// behind an explicit action, so an absent artifact is reported as absent and
// the developer runs the requirement:editor-tasks generate command.

import (
	"os"
	"path/filepath"
	"strings"
)

// GeneratedFragment is the reply. Status says which of the three states this
// is, so a client renders a reason rather than an empty document.
type GeneratedFragment struct {
	// Status is "ok", "absent" when nothing has been generated for the
	// directory, or "missing" when output exists and does not contain this
	// declaration.
	Status string `json:"status"`
	// Declaration is the name the fragment belongs to.
	Declaration string `json:"declaration"`
	// File is the generated file, relative to the project root.
	File string `json:"file,omitempty"`
	Text string `json:"text,omitempty"`
	// Stale reports that the source is newer than the generated file, so what
	// is shown is what the last generation produced rather than what this
	// source would produce now.
	Stale   bool   `json:"stale"`
	Message string `json:"message,omitempty"`
}

// generatedFor finds the emitted Go of one declaration.
//
// The file is found by searching the declaration's own directory rather than
// by computing a name: a page tree and a flat template name their artifacts
// differently, and a computed name that fell behind the generator would report
// "absent" about output that is sitting there.
func generatedFor(project *Project, symbol Symbol) GeneratedFragment {
	fragment := GeneratedFragment{Status: "absent", Declaration: symbol.Name}
	path := filePathOf(symbol.URI)
	if project == nil || path == symbol.URI {
		fragment.Message = "there is no project, so there is nothing generated to show"
		return fragment
	}
	directory := filepath.Dir(path)
	entries, err := os.ReadDir(directory)
	if err != nil {
		fragment.Message = "the declaration's directory could not be read"
		return fragment
	}

	name := symbol.GoFunc
	if name == "" {
		name = symbol.Name
	}
	generated := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_pw_gen.go") {
			continue
		}
		generated = true
		candidate := filepath.Join(directory, entry.Name())
		source, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		text, found := goDeclaration(string(source), name)
		if !found {
			continue
		}
		fragment.Status = "ok"
		fragment.File = filepath.ToSlash(relativeTo(project.Root, candidate))
		fragment.Text = text
		fragment.Stale = newerThan(path, candidate)
		if fragment.Stale {
			fragment.Message = "the source has changed since this was generated"
		}
		return fragment
	}

	if generated {
		fragment.Status = "missing"
		fragment.Message = "the generated output in this directory declares nothing named " + name +
			"; it may predate the declaration, in which case pw generate will add it"
		return fragment
	}
	fragment.Message = "pw generate has not run for this directory"
	return fragment
}

// goDeclaration slices the top-level declaration named name out of Go source.
//
// Generated Go is gofmt output, so a top-level block ends at the first closing
// brace in column one. Parsing it with go/parser would be more exact and would
// also fail on output that is mid-write, which is the state this reads it in.
func goDeclaration(source, name string) (string, bool) {
	lines := strings.Split(source, "\n")
	for index, line := range lines {
		if !declares(line, name) {
			continue
		}
		start := index
		// A doc comment belongs to the declaration it introduces.
		for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "//") {
			start--
		}
		for end := index; end < len(lines); end++ {
			if lines[end] == "}" || lines[end] == ")" {
				return strings.Join(lines[start:end+1], "\n"), true
			}
		}
		return strings.Join(lines[start:], "\n"), true
	}
	return "", false
}

// declares reports whether a line opens a top-level declaration of the name.
func declares(line, name string) bool {
	for _, keyword := range []string{"func ", "type ", "var ", "const "} {
		if !strings.HasPrefix(line, keyword) {
			continue
		}
		rest := strings.TrimPrefix(line, keyword)
		// A method's receiver comes first, so the name is looked for after it.
		if strings.HasPrefix(rest, "(") {
			if close := strings.IndexByte(rest, ')'); close >= 0 {
				rest = strings.TrimSpace(rest[close+1:])
			}
		}
		if strings.HasPrefix(rest, name) &&
			(len(rest) == len(name) || !isWordByte(rest[len(name)])) {
			return true
		}
	}
	return false
}

// newerThan reports whether source was modified after generated.
func newerThan(source, generated string) bool {
	left, err := os.Stat(source)
	if err != nil {
		return false
	}
	right, err := os.Stat(generated)
	if err != nil {
		return false
	}
	return left.ModTime().After(right.ModTime())
}
