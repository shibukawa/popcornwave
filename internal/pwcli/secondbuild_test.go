package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSecondBuildProject lays out a project declaring the fasthttp backend: one
// page tree of templates, which is the smallest thing that produces both a
// shared file and a per-transport one.
func writeSecondBuildProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"pages", filepath.Join("cmd", "fixture")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\nfasthttp = true\n\n[generate]\n"+
			"handlers = []\ntemplates = []\nqueries = []\nconfig = []\npages = [\"pages\"]\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "pages", "layout.pw.html"),
		"package pages\n\nexport component Layout(children: html): html {\n  <main><slot required/></main>\n}\n")
	writeTestFile(t, filepath.Join(root, "pages", "page.pw.html"),
		"package pages\n\nexport component Page(): html {\n  <h1>home</h1>\n}\n")
	return root
}

// The whole wiring in one run: a project that declared the fasthttp backend
// generates both builds, each file belongs to exactly one of them, and running
// again writes nothing.
//
// Idempotence is the property worth a whole-run test. The second transport's
// tree is planned after the per-directory sweep and cleans up after itself, and
// the two steps getting that wrong would not fail anything else: they would
// delete and rewrite the same files forever, and pw generate --check would call
// a freshly generated project stale.
func TestASecondBuildIsGeneratedAndStaysGenerated(t *testing.T) {
	root := writeSecondBuildProject(t)
	generateIn(t, root)

	shared := readGenerated(t, filepath.Join(root, "pages", "layout_pw_gen.go"))
	if constraint, ok := buildConstraint([]byte(shared)); ok {
		t.Errorf("the compiled layout names no transport yet carries %q", constraint)
	}
	if _, err := os.Stat(filepath.Join(root, "pages", "layout_fast_pw_gen.go")); err == nil {
		t.Error("the compiled layout was copied for the second transport although both builds share it")
	}

	first := readGenerated(t, filepath.Join(root, "pages", "routes_pw_gen.go"))
	if constraint, _ := buildConstraint([]byte(first)); constraint != strings.TrimSpace(netHTTPConstraint) {
		t.Errorf("the net/http registry carries %q", constraint)
	}
	second := readGenerated(t, filepath.Join(root, "pages", "routes_fast_pw_gen.go"))
	if constraint, _ := buildConstraint([]byte(second)); constraint != strings.TrimSpace(fastHTTPConstraint) {
		t.Errorf("the fasthttp registry carries %q", constraint)
	}
	if !strings.Contains(second, "pwfastpage.Router") || strings.Contains(second, `"net/http"`) {
		t.Errorf("the fasthttp registry is not written for the second transport:\n%s", second)
	}

	// Running again must plan nothing, and --check must agree.
	if output := generateIn(t, root); strings.TrimSpace(output) != "" {
		t.Errorf("a second run rewrote files:\n%s", output)
	}
	generateIn(t, root, "--check")
}

// Turning the declaration off again takes the second build's files with it,
// rather than leaving a tree that compiles under a tag nothing generates for
// any more.
func TestDroppingTheDeclarationSweepsTheSecondBuild(t *testing.T) {
	root := writeSecondBuildProject(t)
	generateIn(t, root)
	derived := filepath.Join(root, "pages", "routes_fast_pw_gen.go")
	if _, err := os.Stat(derived); err != nil {
		t.Fatalf("the second build was not generated: %v", err)
	}

	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\n"+
			"handlers = []\ntemplates = []\nqueries = []\nconfig = []\npages = [\"pages\"]\n")
	generateIn(t, root)

	if _, err := os.Stat(derived); !os.IsNotExist(err) {
		t.Errorf("the second build survived the declaration being dropped: %v", err)
	}
	if constraint, ok := buildConstraint([]byte(readGenerated(t, filepath.Join(root, "pages", "routes_pw_gen.go")))); ok {
		t.Errorf("the net/http registry still carries %q with no second build declared", constraint)
	}
}

func readGenerated(t *testing.T, path string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	return string(source)
}
