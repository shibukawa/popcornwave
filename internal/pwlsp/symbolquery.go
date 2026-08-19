package pwlsp

// workspace/symbol over the project index.
//
// Answers come from the index for files on disk and from the open documents
// for the ones the editor holds, because a buffer being edited is ahead of the
// file and a symbol search that lands on a stale position is worse than one
// that finds nothing.

import (
	"sort"
	"strings"
	"unicode"
)

// SymbolInformation is the flat workspace/symbol result. The flat shape rather
// than the hierarchical one, because every client accepts it and a workspace
// result has no hierarchy to carry anyway.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// maxSymbolResults bounds one answer. A client renders a list a developer
// scrolls, and an unbounded reply to an empty query would be the whole project.
const maxSymbolResults = 256

// matches reports whether a declaration name answers the query.
//
// Two ways, both of which a developer already expects from their editor: a
// case-insensitive substring, and a camel-hump initialism, so PWL finds
// PageWithLayout. An empty query matches everything, which is how a client
// showing the whole symbol list asks for it.
func matches(name, query string) bool {
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
		return true
	}
	return matchesHumps(name, query)
}

func matchesHumps(name, query string) bool {
	humps := make([]rune, 0, len(name))
	for index, letter := range name {
		if index == 0 || unicode.IsUpper(letter) {
			humps = append(humps, unicode.ToLower(letter))
		}
	}
	if len(humps) == 0 {
		return false
	}
	wanted := []rune(strings.ToLower(query))
	position := 0
	for _, hump := range humps {
		if position < len(wanted) && hump == wanted[position] {
			position++
		}
	}
	return position == len(wanted)
}

// workspaceSymbols answers one query from the index and the open documents.
func workspaceSymbols(query string, built index, open []openSymbols) []SymbolInformation {
	fresh := map[string]bool{}
	for _, document := range open {
		fresh[document.uri] = true
	}

	found := []SymbolInformation{}
	for _, declaration := range built.declarations {
		if fresh[declaration.URI] || !matches(declaration.Name, query) {
			continue
		}
		found = append(found, SymbolInformation{
			Name:          declaration.Name,
			Kind:          declaration.Kind,
			Location:      Location{URI: declaration.URI, Range: declaration.Range},
			ContainerName: declaration.Container,
		})
	}
	for _, document := range open {
		for _, symbol := range document.symbols {
			if !matches(symbol.Name, query) {
				continue
			}
			found = append(found, SymbolInformation{
				Name:          symbol.Name,
				Kind:          symbol.Kind,
				Location:      Location{URI: document.uri, Range: symbol.Range},
				ContainerName: document.container,
			})
		}
	}

	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Name != found[j].Name {
			return found[i].Name < found[j].Name
		}
		return found[i].ContainerName < found[j].ContainerName
	})
	if len(found) > maxSymbolResults {
		found = found[:maxSymbolResults]
	}
	return found
}

// openSymbols is one open document's contribution to a symbol answer.
type openSymbols struct {
	uri       string
	container string
	symbols   []DocumentSymbol
}
