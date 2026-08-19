package pwlsp

// textDocument/references over the type graph.
//
// A reference is found by scanning the sources that can see the declaration
// for its name as a whole word. That is the same resolution rule
// textDocument/definition applies, run in the other direction, and it carries
// the same limit: a word matching a declaration name is reported wherever it is
// written, including inside a string literal. Narrowing that needs the body
// AST rather than the name graph, and is stated rather than hidden.

import (
	"os"
	"sort"
	"strings"
)

type referenceContext struct {
	project *Project
	graph   *TypeGraph
	// open supplies the buffer text for a document the editor holds, so a
	// reference typed and not saved is found where it now is.
	open map[string]string
}

// referencesFor lists every occurrence of a declaration's name in the sources
// that can see it, with the declaration itself included only when asked for.
func referencesFor(context referenceContext, symbol Symbol, includeDeclaration bool) []Location {
	if context.project == nil || context.graph == nil {
		return []Location{}
	}
	found := []Location{}
	for _, uri := range context.searchable(symbol) {
		text, readable := context.textOf(uri)
		if !readable {
			continue
		}
		starts := newLineStarts(text)
		for _, at := range wholeWordRanges(text, starts, symbol.Name) {
			if !includeDeclaration && uri == symbol.URI && at.Start == symbol.Range.Start {
				continue
			}
			found = append(found, Location{URI: uri, Range: at})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].URI != found[j].URI {
			return found[i].URI < found[j].URI
		}
		if found[i].Range.Start.Line != found[j].Range.Start.Line {
			return found[i].Range.Start.Line < found[j].Range.Start.Line
		}
		return found[i].Range.Start.Character < found[j].Range.Start.Character
	})
	return found
}

// searchable is every indexed source from which the declaration is visible.
//
// A file that cannot name the declaration cannot reference it, so scanning it
// would only produce a coincidence: another package's unrelated Card.
func (c referenceContext) searchable(symbol Symbol) []string {
	var uris []string
	for uri := range c.graph.byFile {
		// Two packages may each declare the name. Only a file whose resolution
		// lands on this declaration is looking at this one, which is what
		// keeps another package's unrelated Card out of the answer.
		resolved, found := c.graph.Resolve(uri, symbol.Name)
		if !found || resolved.URI != symbol.URI || resolved.Range.Start != symbol.Range.Start {
			continue
		}
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	return uris
}

func (c referenceContext) textOf(uri string) (string, bool) {
	if text, open := c.open[uri]; open {
		return text, true
	}
	path := filePathOf(uri)
	if path == uri {
		return "", false
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(source), true
}

// wholeWordRanges finds every occurrence of name that is not part of a longer
// identifier, so Card does not match CardList.
func wholeWordRanges(text string, starts lineStarts, name string) []Range {
	if name == "" {
		return nil
	}
	var found []Range
	for offset := 0; ; {
		index := strings.Index(text[offset:], name)
		if index < 0 {
			return found
		}
		start := offset + index
		end := start + len(name)
		offset = end
		if start > 0 && isWordByte(text[start-1]) {
			continue
		}
		if end < len(text) && isWordByte(text[end]) {
			continue
		}
		found = append(found, Range{Start: starts.positionOf(text, start), End: starts.positionOf(text, end)})
	}
}
