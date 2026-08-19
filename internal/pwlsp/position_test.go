package pwlsp

import "testing"

func TestPositionCountsUTF16CodeUnits(t *testing.T) {
	// The one case where bytes, runes, and LSP characters all differ: an emoji
	// is four bytes, one rune, and two UTF-16 units, and a client that trusts
	// a byte count here highlights the wrong span.
	source := "<p>🍿ab</p>\n"
	starts := newLineStarts(source)

	at := starts.positionOf(source, len("<p>🍿a"))
	if at.Line != 0 {
		t.Fatalf("line = %d, want 0", at.Line)
	}
	if at.Character != 6 {
		t.Fatalf("character = %d, want 6 (three ASCII, one emoji as two units)", at.Character)
	}
}

func TestOffsetOfConvertsAOneBasedByteColumn(t *testing.T) {
	source := "one\ntwo\nthree\n"
	starts := newLineStarts(source)

	if got := starts.offsetOf(source, 2, 1); got != 4 {
		t.Fatalf("start of line 2 = %d, want 4", got)
	}
	if got := starts.offsetOf(source, 3, 3); got != 10 {
		t.Fatalf("line 3 column 3 = %d, want 10", got)
	}
}

func TestOffsetPastTheEndClampsToTheDocument(t *testing.T) {
	// A parser reporting a position after the last line is reporting where the
	// missing text belonged, which is the end of the document.
	source := "one\n"
	starts := newLineStarts(source)

	if got := starts.offsetOf(source, 99, 1); got != len(source) {
		t.Fatalf("offset = %d, want %d", got, len(source))
	}
	if got := starts.offsetOf(source, 1, 99); got != len(source) {
		t.Fatalf("offset = %d, want %d", got, len(source))
	}
}

func TestRangeAtMarksTheWordUnderThePosition(t *testing.T) {
	source := "export component Page(): html {\n"
	starts := newLineStarts(source)

	marked := starts.rangeAt(source, len("export "))
	if marked.Start.Character != 7 || marked.End.Character != 16 {
		t.Fatalf("range = %+v, want the word component", marked)
	}
}

func TestRangeAtMarksTheRestOfTheLineWithNoWordThere(t *testing.T) {
	source := "  <p>{x}</p>\n"
	starts := newLineStarts(source)

	marked := starts.rangeAt(source, 2)
	if marked.Start.Character != 2 || marked.End.Character != 12 {
		t.Fatalf("range = %+v, want the rest of the line", marked)
	}
}

func TestRangeAtTheEndOfTheDocumentIsEmpty(t *testing.T) {
	source := "one\n"
	starts := newLineStarts(source)

	marked := starts.rangeAt(source, len(source))
	if marked.Start != marked.End {
		t.Fatalf("range = %+v, want an empty range", marked)
	}
}

func TestLineStartsCountsATrailingNewline(t *testing.T) {
	// A document ending in a newline has a last line that holds nothing, and a
	// position on it is where an appended character would go.
	starts := newLineStarts("a\nb\n")
	if len(starts) != 3 {
		t.Fatalf("line starts = %v, want three", starts)
	}
}
