package pwcli

import (
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
