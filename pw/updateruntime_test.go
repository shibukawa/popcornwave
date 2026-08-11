package pw

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The update runtime is this framework's implementation of the wire contract
// system:tinybind publishes, and it is four hundred lines of JavaScript that no
// Go assertion can reach. This drives it under node against a stubbed page:
// the requests it issues, the responses it consumes, the validator bookkeeping,
// supersession, and every fallback path.
//
// It is the protocol half deliberately, which is the half a browser test would
// be a clumsy way to cover. Real DOM insertion is the browser's job, and what
// this framework can be wrong about on its own is the wire.
func TestUpdateRuntimeConformance(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		// A machine with no node still builds and serves the runtime; it just
		// cannot check it. Failing here would make the toolchain a build
		// dependency of a Go library, which it is not.
		t.Skip("node is not installed, so the browser runtime is unchecked here")
	}
	output, err := exec.Command(node, "testdata/update_harness.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("the update runtime does not conform:\n%s", output)
	}
	if !strings.Contains(string(output), "all checks passed") {
		t.Fatalf("the harness did not report success:\n%s", output)
	}
}

// The signal registry is the boundary half's own bookkeeping: which names
// resolve, which page scopes are on screen, and what a pw-page element's connect
// and disconnect reactions do to both.
//
// It needs its own harness because the update one stubs the boundary half out.
// None of it is visible to a Go assertion, and all of it is what an author would
// otherwise discover from a bug report — a handler that stopped firing after a
// navigation, or one that fired on the wrong page.
func TestSignalRegistryConformance(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed, so the signal registry is unchecked here")
	}
	output, err := exec.Command(node, "testdata/signal_harness.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("the signal registry does not conform:\n%s", output)
	}
	if !strings.Contains(string(output), "all checks passed") {
		t.Fatalf("the harness did not report success:\n%s", output)
	}
}

// The merged asset is one module, and a module that throws at load leaves a page
// with no updates, no boundaries, and nothing in the console pointing at why.
// Parsing it is the cheapest possible guard against that.
func TestTheMergedRuntimeParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed, so the merged asset is unchecked here")
	}
	script := t.TempDir() + "/merged.mjs"
	if err := os.WriteFile(script, []byte(mergedRuntimeScript()), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(node, "--check", script).CombinedOutput(); err != nil {
		t.Fatalf("the merged runtime does not parse:\n%s", output)
	}
}

// The asset carries no dependency bytes any more. This is what the client
// ownership work bought, and a regression would be an upgrade quietly putting a
// second apply implementation back on the page.
func TestTheRuntimeIsEntirelyThisFrameworks(t *testing.T) {
	var source strings.Builder
	for _, name := range []string{"boundary.js", "update.js", "updateboot.js"} {
		part, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(part)
		source.WriteByte('\n')
	}
	merged := source.String()
	// The dependency's own factory, its default header namespace, its default
	// endpoint prefix, and its installed global. A comment naming the module is
	// fine and expected; what must not be here is its code.
	for _, absent := range []string{"createPartialUpdateRuntime", "X-Tinybind", "/_tb", `window["tinybind"]`} {
		if strings.Contains(merged, absent) {
			t.Errorf("the merged asset carries %q, which belongs to the dependency", absent)
		}
	}
	// One apply core, reached from both halves: the boundary path brackets a
	// range, the update path swaps an addressed element, and both carry client
	// state across through the same code.
	if !strings.Contains(merged, "function carryClientState") {
		t.Error("the shared client-state core is missing")
	}
	if strings.Count(merged, "function carryClientState") != 1 {
		t.Error("the shared client-state core is declared more than once")
	}
}
