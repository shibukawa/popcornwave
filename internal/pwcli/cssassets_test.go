package pwcli

import (
	"strings"
	"testing"
)

// TestStylesheetURLRewriteSurvivesParenthesesInAName is the failure this scanner
// was written wrong for at first: taking the first ")" cuts url("a(b).png") in
// half and emits a stylesheet that no longer parses, with a build that
// succeeded and a page that lost its styling.
func TestStylesheetURLRewriteSurvivesParenthesesInAName(t *testing.T) {
	rewrites := map[string]string{"img/logo (1).png": "img/logo (1).webp"}
	source := `.a { background-image: url("img/logo (1).png"); color: red }`
	got := rewriteStylesheetURLs(source, "site.css", rewrites)
	if !strings.Contains(got, `url("img/logo (1).webp")`) {
		t.Errorf("rewrite = %q", got)
	}
	if strings.Count(got, "url(") != 1 || !strings.HasSuffix(got, "color: red }") {
		t.Errorf("the declaration was cut: %q", got)
	}
}

func TestStylesheetURLRewriteLeavesWhatItCannotResolve(t *testing.T) {
	rewrites := map[string]string{"img/a.png": "img/a.webp"}
	for _, source := range []string{
		`.a { background: url(data:image/png;base64,AAAA) }`,
		`.a { background: url(https://cdn.example.com/img/a.png) }`,
		`.a { background: url(//cdn.example.com/img/a.png) }`,
		`.a { background: url(img/unknown.png) }`,
		`.a { background: url(var(--logo)) }`,
	} {
		if got := rewriteStylesheetURLs(source, "site.css", rewrites); got != source {
			t.Errorf("rewriteStylesheetURLs(%q) = %q", source, got)
		}
	}
}

// TestStylesheetURLRewriteResolvesAgainstTheStylesheet covers the rule that a
// relative url() is relative to the file it is written in, never to the page.
func TestStylesheetURLRewriteResolvesAgainstTheStylesheet(t *testing.T) {
	rewrites := map[string]string{"img/a.png": "img/a.webp"}
	got := rewriteStylesheetURLs(`.a{background:url(../img/a.png?v=2)}`, "css/site.css", rewrites)
	if !strings.Contains(got, "url(../img/a.webp?v=2)") {
		t.Errorf("rewrite = %q", got)
	}
}

// TestStylesheetURLRewriteRefusesAnAmbiguousAbsoluteReference keeps the build
// from guessing: the mount prefix is runtime configuration, so an absolute URL
// is matched by suffix, and two sources matching one reference name no file.
func TestStylesheetURLRewriteRefusesAnAmbiguousAbsoluteReference(t *testing.T) {
	rewrites := map[string]string{"a.png": "a.webp", "img/a.png": "img/a.webp"}
	source := `.a{background:url(/public/img/a.png)}`
	if got := rewriteStylesheetURLs(source, "site.css", rewrites); got != source {
		t.Errorf("an ambiguous reference was rewritten: %q", got)
	}
}

func TestAuthoredScriptIsMinifiedOnlyWhenAsked(t *testing.T) {
	source := []byte("export function hello(name) {\n  const greeting = `hi ${name}`;\n  return greeting;\n}\n")
	unchanged, err := transformAuthoredFile("js/app.js", source, assetsConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(source) {
		t.Errorf("a script was minified with the switch off: %q", unchanged)
	}
	minified, err := transformAuthoredFile("js/app.js", source, assetsConfig{Scripts: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(minified) >= len(source) {
		t.Errorf("minified = %q", minified)
	}
	// A transform keeps the module a module: nothing is bundled, so the export
	// and the import graph survive.
	if !strings.Contains(string(minified), "export") {
		t.Errorf("the module lost its export: %q", minified)
	}
}
