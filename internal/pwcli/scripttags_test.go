package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScriptModuleTagIsRequired is the check that makes the module output safe.
//
// The transform cannot see the tag and must not read it, so this is the only
// place the two halves are checked against each other. Without it, a classic
// tag on a built entry is a page that renders and silently loses its script.
func TestScriptModuleTagIsRequired(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "handlers", "home.pw.html"),
		"<div>\n  <h1>hi</h1>\n  <script src=\"/public/js/app.ts\"></script>\n</div>\n")

	err := verifyScriptModuleTags(root)
	if err == nil {
		t.Fatal("a classic tag on a built entry was accepted")
	}
	for _, fragment := range []string{"handlers/home.pw.html:3", "app.ts", "type=\"module\""} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error is missing %q: %v", fragment, err)
		}
	}
}

func TestScriptModuleTagAccepted(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "pages", "page.pw.html"),
		"<script type=\"module\" src=\"/public/js/app.ts\"></script>\n"+
			"<script src='/public/js/legacy.js'></script>\n"+
			"<script type=module src=/public/js/other.ts></script>\n")

	if err := verifyScriptModuleTags(root); err != nil {
		t.Fatalf("a module tag was refused: %v", err)
	}
}

// TestScriptModuleTagIgnoresWhatItDoesNotBuild keeps the check and the hook on
// one definition of a built entry: an authored js is served as written, so its
// tag is the author's business.
func TestScriptModuleTagIgnoresWhatItDoesNotBuild(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "templates", "document.pw.html"),
		"<script src=\"/public/js/vendor.js\"></script>\n"+
			"<script src=\"https://cdn.example.com/x.ts\"></script>\n")

	if err := verifyScriptModuleTags(root); err != nil {
		t.Fatalf("an unbuilt reference was refused: %v", err)
	}
}

func TestTagAttributeReadsTheQuotingStyles(t *testing.T) {
	for _, testcase := range []struct {
		tag       string
		attribute string
		want      string
	}{
		{`<script src="a.ts">`, "src", "a.ts"},
		{`<script src='a.ts'>`, "src", "a.ts"},
		{`<script src=a.ts>`, "src", "a.ts"},
		{`<script  type = "module"  src="a.ts">`, "type", "module"},
		// A near miss that must not be read as the attribute itself.
		{`<script data-type="x" src="a.ts">`, "type", ""},
		{`<script src="a.ts">`, "type", ""},
	} {
		if got := tagAttribute(testcase.tag, testcase.attribute); got != testcase.want {
			t.Errorf("tagAttribute(%q, %q) = %q, want %q",
				testcase.tag, testcase.attribute, got, testcase.want)
		}
	}
}

// TestGenerateRunsTheModuleScan pins where the check lives. It has to run in
// pw generate rather than only in the asset build, so a generate on its own
// reports it and a --check run sees it too.
func TestGenerateRunsTheModuleScan(t *testing.T) {
	source, err := os.ReadFile("generate.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "verifyScriptModuleTags(root)") {
		t.Error("generate.go does not run the module tag scan")
	}
	built, err := os.ReadFile("derivedassets.go")
	if err != nil {
		t.Fatal(err)
	}
	// One place, so a project cannot be told twice or told inconsistently.
	if strings.Contains(string(built), "verifyScriptModuleTags") {
		t.Error("the asset build scans again after generation already did")
	}
}
