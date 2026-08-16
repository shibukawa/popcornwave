package pwmsg

import (
	"fmt"
	"sort"
	"strings"
)

// Piece is one parsed span of a translation.
//
// A translation is plain text: the only markup it may carry is a hole name, and
// a hole stands for an element the template supplies. See
// .knowledge policy:message-rich-text.
type Piece struct {
	// Hole is the hole this piece sits inside, empty at the top level.
	Hole string
	// Arg names the placeholder this piece renders, empty for a literal run.
	Arg string
	// Lit is the literal text, used when Arg is empty.
	Lit string
}

// ParseText splits a translation into pieces.
//
// Placeholders are written {name}. A literal brace is written {{ or }}, matching
// the template language rather than inventing a second escape for the same
// character.
//
// Holes are written <name>text</name> and are recognised only when rich is set,
// so a translation in an ordinary message may contain angle brackets as text.
// That matters because a message is escaped by the template for its position,
// so "a < b" is legitimate content.
func ParseText(text string, rich bool) ([]Piece, error) {
	var pieces []Piece
	var literal strings.Builder
	hole := ""

	flush := func() {
		if literal.Len() == 0 {
			return
		}
		pieces = append(pieces, Piece{Hole: hole, Lit: literal.String()})
		literal.Reset()
	}

	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "{{"):
			literal.WriteByte('{')
			i += 2
		case strings.HasPrefix(text[i:], "}}"):
			literal.WriteByte('}')
			i += 2
		case text[i] == '{':
			end := strings.IndexByte(text[i:], '}')
			if end < 0 {
				return nil, fmt.Errorf("unclosed placeholder in %q", text)
			}
			name := strings.TrimSpace(text[i+1 : i+end])
			if name == "" {
				return nil, fmt.Errorf("empty placeholder in %q", text)
			}
			flush()
			pieces = append(pieces, Piece{Hole: hole, Arg: name})
			i += end + 1
		case rich && strings.HasPrefix(text[i:], "</"):
			end := strings.IndexByte(text[i:], '>')
			if end < 0 {
				return nil, fmt.Errorf("unclosed hole terminator in %q", text)
			}
			name := text[i+2 : i+end]
			if hole == "" {
				return nil, fmt.Errorf("hole %q closed but never opened in %q", name, text)
			}
			if name != hole {
				return nil, fmt.Errorf("hole %q closed by %q in %q", hole, name, text)
			}
			flush()
			hole = ""
			i += end + 1
		case rich && text[i] == '<':
			end := strings.IndexByte(text[i:], '>')
			if end < 0 {
				return nil, fmt.Errorf("unclosed hole in %q", text)
			}
			name := text[i+1 : i+end]
			if name == "" {
				return nil, fmt.Errorf("empty hole name in %q", text)
			}
			if hole != "" {
				return nil, fmt.Errorf("hole %q opened inside hole %q in %q; a hole does not nest, because the template supplies one element per hole", name, hole, text)
			}
			flush()
			hole = name
			i += end + 1
		default:
			literal.WriteByte(text[i])
			i++
		}
	}
	if hole != "" {
		return nil, fmt.Errorf("hole %q left open in %q", hole, text)
	}
	flush()
	return pieces, nil
}

// Placeholders returns the placeholder names a parsed translation uses, sorted.
func Placeholders(pieces []Piece) []string {
	seen := map[string]bool{}
	var names []string
	for _, piece := range pieces {
		if piece.Arg == "" || seen[piece.Arg] {
			continue
		}
		seen[piece.Arg] = true
		names = append(names, piece.Arg)
	}
	sort.Strings(names)
	return names
}

// Holes returns the hole names a parsed translation opens, sorted.
func Holes(pieces []Piece) []string {
	seen := map[string]bool{}
	var names []string
	for _, piece := range pieces {
		if piece.Hole == "" || seen[piece.Hole] {
			continue
		}
		seen[piece.Hole] = true
		names = append(names, piece.Hole)
	}
	sort.Strings(names)
	return names
}
