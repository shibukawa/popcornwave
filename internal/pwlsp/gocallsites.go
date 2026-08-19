package pwlsp

// The Go half of requirement:editor-navigation: from a declaration to the call
// sites of the function it generates, and back.
//
// The names to look for are read out of the generated file rather than derived
// from the declaration. api:cli-generate emits an exported function carrying
// the declaration's name and, depending on the dialect, wrappers around it; the
// scheme is the generator's, and reproducing it here would be a second copy
// that drifts on the next release. Reading what it actually emitted cannot.
//
// A *_pw_gen.go is skipped as a call site. requirement:editor-navigation calls
// it a waypoint and never a destination: policy:generated-artifacts makes it
// uncommitted output, so a result inside one is worthless the moment it is
// regenerated.

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// generatedNames is every exported top-level function the declaration's own
// generated output declares whose name carries the declaration's.
//
// With no generated output the answer is the declaration name alone, which is
// the exported entry every dialect emits; the wrappers are what needs reading.
func generatedNames(symbol Symbol) []string {
	primary := symbol.GoFunc
	if primary == "" {
		primary = symbol.Name
	}
	names := map[string]bool{primary: true}

	path := filePathOf(symbol.URI)
	if path == symbol.URI {
		return []string{primary}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return []string{primary}
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_pw_gen.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(filepath.Dir(path), entry.Name()))
		if err != nil {
			continue
		}
		for _, name := range exportedFuncs(string(source)) {
			if strings.Contains(name, symbol.Name) {
				names[name] = true
			}
		}
	}

	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// exportedFuncs lists the top-level function names of Go source. It is a scan
// rather than a parse because the file it reads may be mid-write, and a parse
// that failed would answer nothing where a scan answers most of it.
func exportedFuncs(source string) []string {
	var names []string
	for _, line := range strings.Split(source, "\n") {
		if !strings.HasPrefix(line, "func ") {
			continue
		}
		rest := strings.TrimPrefix(line, "func ")
		if strings.HasPrefix(rest, "(") {
			// A method belongs to its receiver rather than to a declaration.
			continue
		}
		end := 0
		for end < len(rest) && isWordByte(rest[end]) {
			end++
		}
		if end == 0 {
			continue
		}
		name := rest[:end]
		if name[0] >= 'A' && name[0] <= 'Z' {
			names = append(names, name)
		}
	}
	return names
}

// goCallSites finds where handwritten Go calls what a declaration generated.
func goCallSites(project *Project, symbol Symbol, open map[string]string) []Location {
	found := []Location{}
	if project == nil {
		return found
	}
	names := generatedNames(symbol)

	_ = filepath.WalkDir(project.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if skipDirectory(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_pw_gen.go") {
			return nil
		}
		uri := uriOf(path)
		text, readable := open[uri]
		if !readable {
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			text = string(source)
		}
		starts := newLineStarts(text)
		for _, name := range names {
			for _, at := range wholeWordRanges(text, starts, name) {
				found = append(found, Location{URI: uri, Range: at})
			}
		}
		return nil
	})

	sort.Slice(found, func(i, j int) bool {
		if found[i].URI != found[j].URI {
			return found[i].URI < found[j].URI
		}
		return found[i].Range.Start.Line < found[j].Range.Start.Line
	})
	return found
}

// declarationNamed resolves a name written in handwritten Go back to the
// declaration that generated it.
//
// It answers about a Go document without serving one: gopls owns those, per
// vision:editor-support, so this is a request a client makes on the user's
// behalf rather than a provider registered for the language.
func declarationNamed(graph *TypeGraph, name string) (Symbol, bool) {
	if graph == nil || name == "" {
		return Symbol{}, false
	}
	// The generated entry carries the declaration's name, so an exact match is
	// tried first and a wrapper's prefix only after it.
	for _, symbols := range graph.byPackage {
		for _, symbol := range symbols {
			if symbol.Name == name {
				return symbol, true
			}
		}
	}
	// A wrapper carries the declaration's name inside its own, and so does
	// every shorter declaration whose name happens to be a substring of it:
	// BuildRoomByID contains both RoomByID and Room. The longest match is the
	// one the symbol was derived from, and taking any match would answer with
	// whichever the map happened to yield first.
	best, found := Symbol{}, false
	for _, symbols := range graph.byPackage {
		for _, symbol := range symbols {
			if symbol.Name == "" || !strings.Contains(name, symbol.Name) {
				continue
			}
			if !found || len(symbol.Name) > len(best.Name) {
				best, found = symbol, true
			}
		}
	}
	return best, found
}
