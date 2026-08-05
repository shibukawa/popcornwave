// Command pwfmt is the WebAssembly entry the VS Code extension runs to format
// one buffer. It exists so an editor can reach the tinybind formatter with no
// pw binary, no project, and no network, per decision:formatter-delivery.
//
// The contract is deliberately the same one api:cli-fmt exposes through its
// --stdin filter: a dialect on the command line, the source on stdin, the
// result on stdout, a diagnostic on stderr. When the delegated path lands, the
// extension can swap one for the other without changing its caller.
//
// Build with the sibling build.sh, which pins the TinyGo target and flags the
// committed artifact was produced with.
package main

import (
	"io"
	"os"

	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// usage: pwfmt <html|sql|dynamo> [filename]
//
// The file name is optional and only reaches diagnostics; formatting itself
// never depends on it, because the dialect is already named.
func main() {
	if len(os.Args) < 2 {
		fail("usage: pwfmt <html|sql|dynamo> [filename]")
	}

	format := templatefmt.Format(os.Args[1])
	switch format {
	case templatefmt.HTML, templatefmt.SQL, templatefmt.Dynamo:
	default:
		fail("unknown dialect " + os.Args[1])
	}

	name := "<buffer>"
	if len(os.Args) > 2 && os.Args[2] != "" {
		name = os.Args[2]
	}

	source, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail("cannot read the source: " + err.Error())
	}

	// Options{} is the documented default set. The extension does not expose a
	// width or a whitespace switch, because a buffer formatted differently from
	// the project's own pw fmt is the one outcome worth avoiding.
	formatted, err := templatefmt.SourceAs(format, name, source, templatefmt.Options{})
	if err != nil {
		fail(err.Error())
	}

	if _, err := os.Stdout.Write(formatted); err != nil {
		fail("cannot write the result: " + err.Error())
	}
}

func fail(message string) {
	_, _ = io.WriteString(os.Stderr, message)
	os.Exit(1)
}
