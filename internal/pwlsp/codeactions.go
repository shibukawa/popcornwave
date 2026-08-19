package pwlsp

// textDocument/codeAction.
//
// requirement:editor-code-actions gates every action on a
// decision:shared-check-catalog finding: the editor offers nothing
// api:cli-generate would not have complained about first. That gate is what
// decides the scope here rather than a wish list — the server produces two
// findings, and only one of them has a mechanical repair.
//
// A syntax error has none: what to write is the developer's answer, and an
// action that guessed would be the invented code the requirement excludes.
// A source no purpose compiles has one, and it is an edit to popcornweb.toml
// rather than to the file the developer is looking at.

import (
	"os"
	"path/filepath"
	"strings"
)

// CodeAction is one offer. Kind is the LSP hierarchy a client filters on;
// quickfix is what a lightbulb beside a diagnostic shows.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	// IsPreferred marks the one a client applies for "fix all of this kind".
	IsPreferred bool `json:"isPreferred,omitempty"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

const kindQuickFix = "quickfix"

// codeActionsFor offers a repair for each finding in the requested range.
func codeActionsFor(project *Project, doc *document, within Range, reported []Diagnostic) []CodeAction {
	actions := []CodeAction{}
	if project == nil || doc == nil {
		return actions
	}
	path := filePathOf(doc.uri)
	if path == doc.uri {
		return actions
	}
	for _, finding := range reported {
		if finding.Severity != severityWarning || !overlaps(finding.Range, within) {
			continue
		}
		if action, ok := listDirectoryAction(project, path, doc.kind, finding); ok {
			actions = append(actions, action)
		}
	}
	return actions
}

// listDirectoryAction adds the file's own directory to the purpose that would
// compile it.
//
// The other repair the requirement names for this finding is moving the file,
// and it is not offered: where to move it is a judgement about how the project
// is laid out, and an action that picked a directory would be choosing for the
// developer. Listing the directory they already chose is not a choice.
func listDirectoryAction(project *Project, path string, kind Dialect, finding Diagnostic) (CodeAction, bool) {
	purpose, known := purposeForDialect(kind)
	if !known {
		return CodeAction{}, false
	}
	// A page tree finding is about the file's name rather than its directory,
	// so listing the directory would repair nothing. The message names the
	// reserved names, which is the whole of the answer.
	if strings.Contains(finding.Message, "page tree") {
		return CodeAction{}, false
	}

	directory := filepath.ToSlash(relativeTo(project.Root, filepath.Dir(path)))
	if directory == "." || strings.HasPrefix(directory, "..") {
		return CodeAction{}, false
	}

	source, err := os.ReadFile(project.ConfigPath)
	if err != nil {
		return CodeAction{}, false
	}
	config := string(source)
	at, entry, ok := purposeEntry(config, purpose)
	if !ok {
		return CodeAction{}, false
	}
	inserted, edited := withDirectory(entry, directory)
	if !edited {
		return CodeAction{}, false
	}

	return CodeAction{
		Title:       "List " + directory + " under " + purpose,
		Kind:        kindQuickFix,
		Diagnostics: []Diagnostic{finding},
		IsPreferred: true,
		Edit: &WorkspaceEdit{Changes: map[string][]TextEdit{
			uriOf(project.ConfigPath): {{Range: at, NewText: inserted}},
		}},
	}, true
}

// purposeForDialect is the popcornweb.toml key that compiles a dialect.
func purposeForDialect(kind Dialect) (string, bool) {
	switch kind {
	case dialectHTML:
		return "templates", true
	case dialectSQL:
		return "queries", true
	case dialectDynamo:
		return "dynamo", true
	default:
		return "", false
	}
}

// purposeEntry finds the key's line and returns the range covering its value.
//
// A text scan rather than a parse, and a rewrite of one line rather than of the
// document: the file is the developer's, full of the comments the scaffold
// wrote, and a re-serialized TOML would return it to them reformatted.
func purposeEntry(config, purpose string) (Range, string, bool) {
	starts := newLineStarts(config)
	offset := 0
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		name, value, found := strings.Cut(trimmed, "=")
		if found && strings.TrimSpace(name) == purpose {
			// Only a single-line array is edited. A value spread over several
			// lines is one this cannot rewrite without reformatting it.
			if strings.HasPrefix(strings.TrimSpace(value), "[") && strings.HasSuffix(trimmed, "]") {
				start := offset + strings.Index(line, "=") + 1
				return Range{
					Start: starts.positionOf(config, start),
					End:   starts.positionOf(config, offset+len(line)),
				}, strings.TrimSpace(value), true
			}
			return Range{}, "", false
		}
		offset += len(line) + 1
	}
	return Range{}, "", false
}

// withDirectory returns the array with the directory added, and whether it had
// to be added at all.
func withDirectory(entry, directory string) (string, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(entry, "["), "]")
	quoted := `"` + directory + `"`
	existing := []string{}
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == quoted {
			return "", false
		}
		existing = append(existing, part)
	}
	return " [" + strings.Join(append(existing, quoted), ", ") + "]", true
}

func overlaps(finding, within Range) bool {
	return finding.Start.Line <= within.End.Line && finding.End.Line >= within.Start.Line
}
