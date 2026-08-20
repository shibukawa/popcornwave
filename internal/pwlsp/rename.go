package pwlsp

// The edit set of requirement:declaration-rename.
//
// A declaration name decides more than the declaration: the exported Go
// function api:cli-generate emits, the handwritten Go that calls it, and every
// template reference to it. That is why requirement:pw-language-server defers
// rename rather than serving it from the parse — and why the set is computed
// once, here, for both the editor and system:pw-cli to use.
//
// Nothing is written. The caller previews it or applies it, because the set
// crosses files the developer did not open.
//
// No route moves. requirement:declaration-rename lists a route among what a
// name decides, and that is the page tree's directory name rather than the
// declaration inside it: renaming the declaration in a page.pw.html leaves the
// URL exactly where it was. Renaming the directory is a different operation,
// and this is not it.

import (
	"errors"
	"sort"
	"strings"
	"unicode"
)

// RenamePlan is what a rename would do.
type RenamePlan struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Changes are the edits, grouped by document.
	Changes map[string][]TextEdit `json:"changes"`
	// Refusals are the reasons this rename must not proceed. A plan with any
	// refusal is not applied: a partial rename leaves a project that does not
	// compile, and rule:page-directory-naming and the collision below are both
	// knowable before anything is written.
	Refusals []string `json:"refusals,omitempty"`
	// GoCallSites counts the handwritten Go this edits, so a preview can say
	// how far outside the template the rename reaches.
	GoCallSites int `json:"goCallSites"`
}

// Empty reports a plan that would change nothing.
func (p RenamePlan) Empty() bool { return len(p.Changes) == 0 }

// planRename computes the edit set for renaming the declaration at a position.
func planRename(context referenceContext, symbol Symbol, to string) RenamePlan {
	plan := RenamePlan{From: symbol.Name, To: to, Changes: map[string][]TextEdit{}}

	if reason, ok := refuseName(symbol, to); !ok {
		plan.Refusals = append(plan.Refusals, reason)
	}
	if context.graph != nil {
		if existing, taken := context.graph.Resolve(symbol.URI, to); taken {
			plan.Refusals = append(plan.Refusals,
				to+" is already declared in "+existing.Container)
		}
	}

	// The template side: the declaration and every reference the graph says
	// can see it, which is the same set find-references answers with.
	for _, at := range referencesFor(context, symbol, true) {
		plan.Changes[at.URI] = append(plan.Changes[at.URI], TextEdit{Range: at.Range, NewText: to})
	}

	// The Go side. A generated file is not edited: policy:generated-artifacts
	// makes it output, and the next generation writes the new name itself.
	for _, at := range goCallSites(context.project, symbol, context.open) {
		plan.Changes[at.URI] = append(plan.Changes[at.URI], TextEdit{Range: at.Range, NewText: to})
		plan.GoCallSites++
	}

	for uri := range plan.Changes {
		edits := plan.Changes[uri]
		// Descending, so applying one edit does not move the next.
		sort.Slice(edits, func(i, j int) bool {
			if edits[i].Range.Start.Line != edits[j].Range.Start.Line {
				return edits[i].Range.Start.Line > edits[j].Range.Start.Line
			}
			return edits[i].Range.Start.Character > edits[j].Range.Start.Character
		})
		plan.Changes[uri] = edits
	}

	return plan
}

// refuseName reports whether the new name is one this language accepts.
//
// A component and a statement are PascalCase, which the parser enforces; a
// rename producing a name it would refuse is a rename that breaks the file,
// and finding that out after the write is worse than being told now.
func refuseName(symbol Symbol, to string) (string, bool) {
	switch {
	case to == "":
		return "a name is required", false
	case to == symbol.Name:
		return "the new name is the old one", false
	case !isIdentifier(to):
		return to + " is not an identifier", false
	}
	switch symbol.Kind {
	case kindComponent, kindStatement, kindType, kindEnum:
		if !unicode.IsUpper([]rune(to)[0]) {
			return to + " must be PascalCase, which is what the parser accepts for a " + string(symbol.Kind), false
		}
	}
	return "", true
}

func isIdentifier(name string) bool {
	for index, letter := range name {
		switch {
		case unicode.IsLetter(letter), letter == '_':
		case unicode.IsDigit(letter) && index > 0:
		default:
			return false
		}
	}
	return name != ""
}

// Summary is the one-line description a preview leads with.
func (p RenamePlan) Summary() string {
	if len(p.Refusals) > 0 {
		return strings.Join(p.Refusals, "; ")
	}
	files := len(p.Changes)
	edits := 0
	for _, changes := range p.Changes {
		edits += len(changes)
	}
	return countOf(edits, "edit") + " in " + countOf(files, "file") +
		", of which " + countOf(p.GoCallSites, "call site") + " in handwritten Go"
}

// PlanRenameIn is the requirement:declaration-rename entry point for a caller
// with no open documents, which is system:pw-cli.
//
// The editor and the command compute one edit set from one implementation,
// which is what keeps the two from renaming differently.
func PlanRenameIn(project *Project, from, to string) (RenamePlan, error) {
	built := buildIndex(project)
	graph := built.graph
	var symbol Symbol
	found := false
	for uri := range graph.byFile {
		if resolved, ok := graph.Resolve(uri, from); ok && resolved.Name == from {
			symbol, found = resolved, true
			break
		}
	}
	if !found {
		return RenamePlan{}, errors.New("no declaration named " + from)
	}
	return planRename(referenceContext{project: project, graph: graph}, symbol, to), nil
}

// PathOf is the file a document URI names, for a caller outside this package.
func PathOf(uri string) string { return filePathOf(uri) }

// OffsetsOf converts a range into byte offsets in text, so a caller applying an
// edit does not reimplement the UTF-16 accounting an LSP position carries.
func OffsetsOf(text string, at Range) (int, int) {
	starts := newLineStarts(text)
	return starts.offsetOfPosition(text, at.Start), starts.offsetOfPosition(text, at.End)
}
