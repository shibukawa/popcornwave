package pwlsp

// textDocument/inlayHint.
//
// requirement:editor-inlay-hints exists because these sources name almost none
// of the types the generator resolves: a val binding writes no type at all, and
// the developer's alternative is to read the generated Go.
//
// A hint appears only where the type was resolved. An unresolved position shows
// nothing rather than a guess, which is the rule every other resolved answer
// here follows.

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// InlayHint is one hint. Kind 1 is a type hint, which is what all of these are.
type InlayHint struct {
	Position    Position `json:"position"`
	Label       string   `json:"label"`
	Kind        int      `json:"kind"`
	PaddingLeft bool     `json:"paddingLeft,omitempty"`
	Tooltip     string   `json:"tooltip,omitempty"`
}

const inlayTypeHint = 1

// maxHintLabel bounds one label. A long generic type would otherwise reflow
// the line it annotates, which is the one thing a hint must not do.
const maxHintLabel = 40

// inlayHints answers one request over a line range.
//
// The families are switchable, so a developer who knows the schema gets
// different noise from one learning it. Off means no work rather than no
// display: an unwanted family should not cost a walk.
func inlayHints(found analysis, text string, starts lineStarts, within Range, graph *TypeGraph, uri string, enabled map[BindingKind]bool) []InlayHint {
	hints := []InlayHint{}
	if found.module == nil {
		return hints
	}
	for _, declaration := range found.module.Declarations {
		template, ok := declaration.(*sqlbind.TemplateDecl)
		if !ok {
			continue
		}
		for _, binding := range bindingsIn(template, graph, uri) {
			if !enabled[binding.Kind] || binding.Type == "" {
				continue
			}
			// A parameter writes its own type. Annotating it would repeat the
			// source back at the reader.
			if binding.Kind == bindingParameter {
				continue
			}
			at := hintPosition(text, starts, binding)
			if at.Line < within.Start.Line || at.Line > within.End.Line {
				continue
			}
			hints = append(hints, InlayHint{
				Position:    at,
				Label:       ": " + shorten(binding.Type),
				Kind:        inlayTypeHint,
				PaddingLeft: false,
				Tooltip:     binding.Origin + ", resolved as " + binding.Type,
			})
		}
	}
	return hints
}

// hintPosition is just after the bound name, which is where a type would have
// been written if the language asked for one.
func hintPosition(text string, starts lineStarts, binding Binding) Position {
	offset := starts.offsetOf(text, binding.Pos.Line, binding.Pos.Col)
	// The recorded position is the start of the construct rather than of the
	// name for a loop, so the name is found from there.
	index := strings.Index(text[offset:], binding.Name)
	if index < 0 {
		return starts.positionOf(text, offset)
	}
	return starts.positionOf(text, offset+index+len(binding.Name))
}

// shorten bounds a label, counting characters rather than bytes: the cap is
// about how much of the line the hint covers, and a multi-byte name covers no
// more of it than an ASCII one.
func shorten(label string) string {
	runes := []rune(label)
	if len(runes) <= maxHintLabel {
		return label
	}
	return string(runes[:maxHintLabel-1]) + "…"
}

// defaultHintFamilies is what a developer gets without configuring anything:
// the types the source never writes, and not the ones it does.
func defaultHintFamilies() map[BindingKind]bool {
	return map[BindingKind]bool{
		bindingVal:       true,
		bindingLoop:      true,
		bindingAwait:     true,
		bindingParameter: false,
	}
}
