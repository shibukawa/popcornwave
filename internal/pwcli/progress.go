package pwcli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// progressLines bounds the region a long phase paints in. Three is enough to
// show the phase and the last thing it said, and small enough that the region
// never becomes the scrollback it is meant to keep clear.
const progressLines = 3

// progressRegion reports the work an operator is waiting on, in a fixed number
// of lines that are rewritten in place and then cleared. A phase that can outlast
// a second gets one, because a slow run and a hung run are otherwise the same
// thing to look at.
//
// What the region never holds is a diagnostic. A warning or an error leaves it
// and stays in the scrollback, since the point is to hide routine progress and
// nothing else.
type progressRegion struct {
	out io.Writer
	// live is false when the output is not a terminal, which turns the region
	// into one plain line per phase so a log file and a CI transcript stay
	// readable.
	live    bool
	phase   string
	details []string
	painted int
}

// newProgressRegion starts a region on out. A caller that never calls Phase
// prints nothing, so it is safe to create one and then take a path that has no
// long work in it.
func newProgressRegion(out io.Writer) *progressRegion {
	return &progressRegion{out: out, live: terminalWriter(out)}
}

// terminalWriter reports whether cursor movement reaches a terminal. Anything
// else — a pipe, a file, a test buffer — gets the plain form.
func terminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}

// Phase names what is happening now, replacing whatever the region held.
func (r *progressRegion) Phase(name string) {
	if r == nil {
		return
	}
	r.phase, r.details = name, nil
	if !r.live {
		fmt.Fprintf(r.out, "pw: %s\n", name)
		return
	}
	r.paint()
}

// Detail adds a line under the phase, keeping only the most recent ones. It is
// for progress a phase produces as it goes, never for something the operator
// has to act on.
func (r *progressRegion) Detail(line string) {
	if r == nil || !r.live || strings.TrimSpace(line) == "" {
		return
	}
	r.details = append(r.details, line)
	if len(r.details) > progressLines-1 {
		r.details = r.details[len(r.details)-(progressLines-1):]
	}
	r.paint()
}

// Done clears the region. What the command has to say about the finished work
// is printed after this, in the scrollback, where it stays.
func (r *progressRegion) Done() {
	if r == nil || !r.live {
		return
	}
	r.clear()
	r.phase, r.details, r.painted = "", nil, 0
}

// paint rewrites the region in place.
func (r *progressRegion) paint() {
	r.clear()
	lines := append([]string{"⋯ " + r.phase}, r.details...)
	for _, line := range lines {
		fmt.Fprintf(r.out, "\033[2K%s\n", truncateLine(line))
	}
	r.painted = len(lines)
}

// clear moves back over the painted lines and erases them, so the next paint
// starts where the last one did rather than below it.
func (r *progressRegion) clear() {
	for range r.painted {
		fmt.Fprint(r.out, "\033[1A\033[2K")
	}
	r.painted = 0
}

// truncateLine keeps a line inside one terminal row, because a wrapped line
// occupies two rows and the cursor arithmetic above counts one.
func truncateLine(line string) string {
	const width = 100
	runes := []rune(strings.ReplaceAll(line, "\n", " "))
	if len(runes) <= width {
		return string(runes)
	}
	return string(runes[:width-1]) + "…"
}
