package pwcli

import (
	"strings"
	"testing"

	templatehtmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// The resolver is what makes a template naming a handler a checked reference
// rather than a string nobody reads. These cover the three answers it can give,
// because each one means something different to the compile that consumes it.

// A block whose shape is read publishes exactly what it returned, and a name the
// markup asked for that is not in it comes back refused with a reason.
func TestScriptResolverAnswersWhatTheBlockReturned(t *testing.T) {
	answers, err := resolveComponentScripts("page.pw.html", []templatehtmlbind.ComponentScript{{
		Component:  "Card",
		Script:     `export function setup({ el, props: { label, missing } }) { return { open, close }; }`,
		Handlers:   []string{"open", "shut"},
		Parameters: []string{"label", "count"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	set := answers.Handlers["Card"]
	if strings.Join(set.Resolved, ",") != "open,close" {
		t.Errorf("resolved %v, want the two the block returned", set.Resolved)
	}
	if reason := set.Unresolved["shut"]; reason == "" {
		t.Error("a referenced name the block does not return was not refused")
	}
	if _, refused := set.Unresolved["open"]; refused {
		t.Error("a name the block does return was refused")
	}
	// Only parameters the component declares. A block asking for one it has not
	// got would otherwise emit an object naming nothing.
	if got := strings.Join(answers.Parameters["Card"], ","); got != "label" {
		t.Errorf("parameters are %q, want only the declared one", got)
	}
}

// A block this scanner cannot understand leaves the component unchecked rather
// than failing the build, because the limit is the scanner's and an author who
// wrote working JavaScript should not be stopped by it.
func TestScriptResolverLeavesAnUnreadBlockUnchecked(t *testing.T) {
	answers, err := resolveComponentScripts("page.pw.html", []templatehtmlbind.ComponentScript{{
		Component: "Card",
		Script:    `export function setup({ el }) { const handlers = { open }; return handlers; }`,
		Handlers:  []string{"open"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := answers.Handlers["Card"]; present {
		t.Error("an unread block was answered for, so its names would be checked against nothing")
	}
}

// A block that is not walkable JavaScript at all is an error, because that is
// not a limit of this scanner: the browser meets the same source next.
func TestScriptResolverRefusesUnwalkableJavaScript(t *testing.T) {
	_, err := resolveComponentScripts("page.pw.html", []templatehtmlbind.ComponentScript{{
		Component: "Card",
		Script:    `export function setup({ el }) { const broken = "never closed; }`,
	}})
	if err == nil {
		t.Fatal("an unterminated string was accepted")
	}
	if !strings.Contains(err.Error(), "Card") {
		t.Errorf("the error does not name the component: %v", err)
	}
}
