package pwcheck

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update rewrites the checked-in documentation page from the catalog:
//
//	go test ./internal/pwcheck -run TestDiagnosticsPage -update
var update = flag.Bool("update", false, "rewrite the generated diagnostics page")

const diagnosticsPage = "../../website/src/content/docs/appendix/diagnostics.md"

// Every identifier a report prints has to resolve to something a reader can
// open, so the page is generated from the catalog and this test is what keeps
// the two from drifting.
func TestDiagnosticsPageIsGeneratedFromTheCatalog(t *testing.T) {
	generated := Markdown()
	path := filepath.FromSlash(diagnosticsPage)
	if *update {
		if err := os.WriteFile(path, []byte(generated), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("wrote", path)
		return
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\nrun: go test ./internal/pwcheck -run TestDiagnosticsPage -update", err)
	}
	if string(current) != generated {
		t.Fatal("the diagnostics page is out of date; run: go test ./internal/pwcheck -run TestDiagnosticsPage -update")
	}
}

// A finding prints its documentation link, so every identifier must have a
// heading on the page to land on.
func TestEveryCheckHasAnEntryAndAnAnchor(t *testing.T) {
	page := Markdown()
	for _, check := range All() {
		heading := "### " + check.ID + ": " + check.Title
		if !strings.Contains(page, heading) {
			t.Errorf("the page has no entry for %s", check.ID)
		}
		anchor := check.DocsURL()
		if !strings.HasPrefix(anchor, DocsBase+"#") {
			t.Errorf("%s links outside the diagnostics page: %s", check.ID, anchor)
		}
		if !strings.Contains(anchor, strings.ToLower(check.ID)) {
			t.Errorf("%s anchor %q does not carry its identifier", check.ID, anchor)
		}
	}
}
