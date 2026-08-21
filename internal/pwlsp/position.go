package pwlsp

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Position is an LSP position: a zero-based line, and a character counted in
// UTF-16 code units from the start of that line. The parsers report a
// one-based line and a one-based byte column, so every position crossing the
// boundary is converted here rather than at each call site.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// lineStarts records the byte offset each line begins at, so a conversion is a
// slice index rather than a scan of the whole document. It is built once per
// document version.
type lineStarts []int

func newLineStarts(source string) lineStarts {
	starts := lineStarts{0}
	for offset := 0; ; {
		index := strings.IndexByte(source[offset:], '\n')
		if index < 0 {
			return starts
		}
		offset += index + 1
		starts = append(starts, offset)
	}
}

// offsetOf converts a one-based line and one-based byte column into a byte
// offset, clamped to the document. A parser reporting a position past the end
// of the source is reporting where the missing text should have been, so the
// end of the document is the honest answer rather than an error.
func (s lineStarts) offsetOf(source string, line, column int) int {
	if line < 1 {
		line = 1
	}
	if line > len(s) {
		return len(source)
	}
	offset := s[line-1]
	if column > 1 {
		offset += column - 1
	}
	if offset > len(source) {
		return len(source)
	}
	return offset
}

// positionOf converts a byte offset into an LSP position.
func (s lineStarts) positionOf(source string, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	// The last line whose start is at or before the offset, found by the
	// binary search the sorted starts exist for — a reference scan converts
	// one position per match, and the walk made that matches × lines.
	line := sort.SearchInts(s, offset+1) - 1
	if line < 0 {
		line = 0
	}
	return Position{Line: line, Character: utf16Len(source[s[line]:offset])}
}

// utf16Len counts a byte span in the UTF-16 code units an LSP character offset
// is measured in. A rune outside the basic plane counts as two, which is the
// one case where a byte count, a rune count, and this answer all differ.
func utf16Len(text string) int {
	units := 0
	for _, r := range text {
		units++
		if r > 0xFFFF {
			units++
		}
	}
	return units
}

// rangeAt turns a reported point into a range a client can underline. The word
// at the position is preferred, because that is what the reader has to look
// at; with no word there, the rest of the line is marked, and at the end of a
// line the range is empty rather than reaching into the next one.
func (s lineStarts) rangeAt(source string, offset int) Range {
	start := s.positionOf(source, offset)
	end := offset
	for end < len(source) && isWordByte(source[end]) {
		end++
	}
	if end == offset {
		for end < len(source) && source[end] != '\n' && source[end] != '\r' {
			end++
		}
	}
	return Range{Start: start, End: s.positionOf(source, end)}
}

func isWordByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '_', b >= utf8.RuneSelf:
		return true
	default:
		return false
	}
}

// offsetOfPosition converts an LSP position back into a byte offset, which is
// what a request carrying a caret needs before anything can be read at it.
func (s lineStarts) offsetOfPosition(source string, at Position) int {
	if at.Line < 0 {
		return 0
	}
	if at.Line >= len(s) {
		return len(source)
	}
	start := s[at.Line]
	end := len(source)
	if at.Line+1 < len(s) {
		end = s[at.Line+1]
	}
	// The character is counted in UTF-16 units, so the line is walked rather
	// than indexed: a rune outside the basic plane counts as two.
	units := 0
	for offset, letter := range source[start:end] {
		if units >= at.Character {
			return start + offset
		}
		units++
		if letter > 0xFFFF {
			units++
		}
	}
	return end
}
