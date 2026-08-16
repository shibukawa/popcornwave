package pwmsg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write lays out a catalog directory and returns its path.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func load(t *testing.T, files map[string]string, locales []string, def string) *Catalog {
	t.Helper()
	catalog, err := Load(write(t, files), locales, def)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return catalog
}

func generate(t *testing.T, catalog *Catalog) *Generated {
	t.Helper()
	out, err := Generate(catalog, "messages")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func TestArgumentlessMessageIsAPlainStringTable(t *testing.T) {
	catalog := load(t, map[string]string{"about.yaml": `
title:
  locales:
    ja: "ようこそ"
    en: "Welcome"
`}, []string{"ja", "en"}, "ja")

	out := generate(t, catalog)
	source := string(out.Source)

	// The point of the shape: a message with no argument reads a string table
	// directly, so rendering it allocates nothing.
	if !strings.Contains(source, "var tblAboutTitle = [...]string{") {
		t.Errorf("expected a plain string table, got:\n%s", source)
	}
	if strings.Contains(source, "RenderMessage(tblAboutTitle") {
		t.Error("an argumentless message should not reach the renderer")
	}
	if got := out.Symbols["about.title"]; got.Name != "AboutTitle" || len(got.Params) != 0 {
		t.Errorf("symbol = %+v, want AboutTitle with no params", got)
	}
}

func TestArgumentsBecomeSegmentsInDeclarationOrder(t *testing.T) {
	catalog := load(t, map[string]string{"about.yaml": `
greeting:
  params: ["name string"]
  locales:
    ja: "ようこそ、{name}さん"
    en: "Welcome, {name}!"
`}, []string{"ja", "en"}, "ja")

	out := generate(t, catalog)
	source := string(out.Source)

	if !strings.Contains(source, `{Lit: "ようこそ、"}, {Arg: 1}, {Lit: "さん"}`) {
		t.Errorf("ja row is not the expected segment list:\n%s", source)
	}
	if !strings.Contains(source, "func AboutGreeting(loc pw.Locale, name string) string") {
		t.Errorf("unexpected signature:\n%s", source)
	}
	if got := out.Symbols["about.greeting"].Params; len(got) != 1 || got[0] != "name" {
		t.Errorf("params = %v, want [name]", got)
	}
}

// The placeholder order differing between locales is the case the segment shape
// exists for: each row lists its own order, so nothing reorders at render.
func TestPlaceholderOrderIsPerLocale(t *testing.T) {
	catalog := load(t, map[string]string{"m.yaml": `
moved:
  params: ["from string", "to string"]
  locales:
    en: "Moved {from} to {to}"
    ja: "{to}へ{from}から移動しました"
`}, []string{"en", "ja"}, "en")

	source := string(generate(t, catalog).Source)
	if !strings.Contains(source, `{Lit: "Moved "}, {Arg: 1}, {Lit: " to "}, {Arg: 2}`) {
		t.Errorf("en row lost its order:\n%s", source)
	}
	if !strings.Contains(source, `{Arg: 2}, {Lit: "へ"}, {Arg: 1}`) {
		t.Errorf("ja row did not keep its own order:\n%s", source)
	}
}

func TestPluralTableHasOneRowPerLocaleCategory(t *testing.T) {
	catalog := load(t, map[string]string{"cart.yaml": `
item-count:
  params: ["n int"]
  plural: n
  locales:
    ja: "{n}件"
    en:
      one: "{n} item"
      other: "{n} items"
`}, []string{"ja", "en"}, "ja")

	out := generate(t, catalog)
	source := string(out.Source)

	if !strings.Contains(source, "var tblCartItemCount = [...][][]pwruntime.Segment{") {
		t.Errorf("expected a two dimensional plural table:\n%s", source)
	}
	// Japanese distinguishes one form and English two, so the rows differ in
	// length. That is the property that makes the category set a fact about the
	// target language rather than about the message.
	if !strings.Contains(source, `{{{Arg: 1}, {Lit: "件"}}},`) {
		t.Errorf("ja row should hold exactly one form:\n%s", source)
	}
	if !strings.Contains(source, "pluralIndex(loc, n)") {
		t.Errorf("plural message should select by category:\n%s", source)
	}
	if !strings.Contains(source, "case 1: // en") {
		t.Errorf("selector should carry a case per locale:\n%s", source)
	}
}

// A single-locale project should carry no plural arithmetic at all.
func TestSingleLocaleCollapsesTheSelector(t *testing.T) {
	catalog := load(t, map[string]string{"cart.yaml": `
item-count:
  params: ["n int"]
  plural: n
  locales:
    ja: "{n}件"
`}, []string{"ja"}, "ja")

	source := string(generate(t, catalog).Source)
	if strings.Contains(source, "pluralEastSlavic") || strings.Contains(source, "n == 1") {
		t.Errorf("a Japanese-only project should carry no plural arithmetic:\n%s", source)
	}
	if !strings.Contains(source, "case 0: // ja") || !strings.Contains(source, "return 0") {
		t.Errorf("expected a constant selector:\n%s", source)
	}
}

func TestOnlyReachedPluralHelpersAreEmitted(t *testing.T) {
	files := map[string]string{"cart.yaml": `
item-count:
  params: ["n int"]
  plural: n
  locales:
    en:
      one: "{n} item"
      other: "{n} items"
    ru:
      one: "{n} товар"
      few: "{n} товара"
      many: "{n} товаров"
`}
	source := string(generate(t, load(t, files, []string{"en", "ru"}, "en")).Source)
	if !strings.Contains(source, "func pluralEastSlavic(") {
		t.Error("a Russian locale should pull its own helper")
	}
	if strings.Contains(source, "func pluralArabic(") {
		t.Error("an unreached helper should not be emitted")
	}
}

// Fallback is resolved while generating, so no row is empty and no runtime
// branch decides what a missing translation renders.
func TestFallbackIsFlattenedIntoTheTable(t *testing.T) {
	catalog := load(t, map[string]string{"a.yaml": `
hello:
  locales:
    ja: "こんにちは"
`}, []string{"ja", "en"}, "ja")

	source := string(generate(t, catalog).Source)
	if strings.Count(source, `"こんにちは"`) != 2 {
		t.Errorf("en row should have been filled from the default locale:\n%s", source)
	}
	if strings.Contains(source, "fallback") {
		t.Errorf("no runtime fallback should survive generation:\n%s", source)
	}
}

func TestRegionFallsBackToItsBaseLanguage(t *testing.T) {
	catalog := load(t, map[string]string{"a.yaml": `
hello:
  locales:
    ja: "こんにちは"
    en: "Hello"
`}, []string{"ja", "en-GB"}, "ja")

	source := string(generate(t, catalog).Source)
	if !strings.Contains(source, `"Hello",`) {
		t.Errorf("en-GB should resolve through en, not through the default:\n%s", source)
	}
}

func TestRichMessageBecomesSegmentsWithHoles(t *testing.T) {
	catalog := load(t, map[string]string{"terms.yaml": `
agree:
  rich: true
  locales:
    ja: "利用規約に同意の上、<a>開始</a>してください"
    en: "Please <a>get started</a> after agreeing to the terms"
`}, []string{"ja", "en"}, "ja")

	out := generate(t, catalog)
	source := string(out.Source)

	if !strings.Contains(source, "func TermsAgree(loc pw.Locale) []htmlbind.MessageSegment") {
		t.Errorf("a rich message returns segments:\n%s", source)
	}
	if !strings.Contains(source, `{Hole: "a", Lit: "開始"}`) {
		t.Errorf("the hole should carry the text inside it:\n%s", source)
	}
	// The word order differing between the two is the whole reason the form
	// exists: the template writes one anchor and the translation decides where
	// it lands.
	if !strings.Contains(source, `{Lit: "Please "}, {Hole: "a", Lit: "get started"}`) {
		t.Errorf("en should open its hole mid-sentence:\n%s", source)
	}
	if !out.Symbols["terms.agree"].Rich {
		t.Error("symbol should be marked rich")
	}
}

func TestIDBecomesOneSymbol(t *testing.T) {
	out := generate(t, load(t, map[string]string{"a.yaml": `
item-count:
  locales: {ja: "件"}
`}, []string{"ja"}, "ja"))
	if got := out.Symbols["a.item-count"].Name; got != "AItemCount" {
		t.Errorf("symbol = %q, want AItemCount", got)
	}
}

func TestSymbolCollisionIsReported(t *testing.T) {
	_, err := Generate(load(t, map[string]string{"a.yaml": `
item-count:
  locales: {ja: "x"}
item_count:
  locales: {ja: "y"}
`}, []string{"ja"}, "ja"), "messages")
	if err == nil || !strings.Contains(err.Error(), "Go symbol") {
		t.Fatalf("expected a collision report, got %v", err)
	}
}

func TestValidation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		locales []string
		want    string
	}{{
		name: "undeclared placeholder",
		body: `
greeting:
  params: ["name string"]
  locales:
    ja: "ようこそ、{nombre}さん"
`,
		locales: []string{"ja"},
		want:    "does not declare",
	}, {
		name: "dropped placeholder",
		body: `
greeting:
  params: ["name string"]
  locales:
    ja: "ようこそ"
`,
		locales: []string{"ja"},
		want:    "never uses declared parameter",
	}, {
		name: "missing plural category",
		body: `
count:
  params: ["n int"]
  plural: n
  locales:
    en:
      other: "{n} items"
`,
		locales: []string{"en"},
		want:    "no one variant",
	}, {
		name: "plural on an untabulated locale",
		body: `
count:
  params: ["n int"]
  plural: n
  locales:
    xx:
      one: "{n}"
      other: "{n}"
`,
		locales: []string{"xx"},
		want:    "no plural rule in this build",
	}, {
		name: "plural driven by a missing parameter",
		body: `
count:
  params: ["n int"]
  plural: quantity
  locales:
    ja: "{n}件"
`,
		locales: []string{"ja"},
		want:    "not a declared parameter",
	}, {
		name: "hole sets disagree",
		body: `
agree:
  rich: true
  locales:
    ja: "<a>開始</a>"
    en: "<b>start</b>"
`,
		locales: []string{"ja", "en"},
		want:    "every locale opens the same set",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			catalog := load(t, map[string]string{"m.yaml": tc.body}, tc.locales, tc.locales[0])
			var found bool
			for _, d := range Validate(catalog, Error) {
				if d.Severity == Error && strings.Contains(d.Message, tc.want) {
					found = true
				}
			}
			if !found {
				t.Errorf("no error containing %q; got %v", tc.want, Validate(catalog, Error))
			}
		})
	}
}

func TestMissingTranslationSeverityIsTheProjectsChoice(t *testing.T) {
	catalog := load(t, map[string]string{"m.yaml": `
hello:
  locales: {ja: "こんにちは"}
`}, []string{"ja", "en"}, "ja")

	for _, severity := range []Severity{Error, Warning} {
		var seen bool
		for _, d := range Validate(catalog, severity) {
			if strings.Contains(d.Message, "supplies no translation") {
				seen = true
				if d.Severity != severity {
					t.Errorf("severity = %v, want %v", d.Severity, severity)
				}
			}
		}
		if !seen {
			t.Errorf("%v: missing translation was not reported", severity)
		}
	}
}

func TestSourceDriftMarksTranslationsStale(t *testing.T) {
	catalog := load(t, map[string]string{"m.yaml": `
hello:
  snapshot: "こんにちは"
  locales:
    ja: "こんばんは"
    en: "Good evening"
`}, []string{"ja", "en"}, "ja")

	var seen bool
	for _, d := range Validate(catalog, Error) {
		if strings.Contains(d.Message, "stale") {
			seen = true
			if d.Severity != Warning {
				t.Errorf("drift should be a warning, got %v", d.Severity)
			}
		}
	}
	if !seen {
		t.Error("a source text differing from its snapshot should be reported")
	}
}

func TestIDLexicalFormMatchesWhatAReferenceAccepts(t *testing.T) {
	for _, id := range []string{"-lead", "trail-", "has space", "has.", "dot..dot"} {
		if err := validateID(id); err == nil {
			t.Errorf("validateID(%q) = nil, want an error", id)
		}
	}
	for _, id := range []string{"item-count", "a.b.c", "with_underscore", "n1"} {
		if err := validateID(id); err != nil {
			t.Errorf("validateID(%q) = %v, want nil", id, err)
		}
	}
}

func TestTextEscapes(t *testing.T) {
	pieces, err := ParseText("a {{literal}} brace and {arg}", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := literalText(pieces); got != "a {literal} brace and " {
		t.Errorf("literal = %q", got)
	}
	if got := Placeholders(pieces); len(got) != 1 || got[0] != "arg" {
		t.Errorf("placeholders = %v", got)
	}
}

// Angle brackets are ordinary text in a message that is not rich, because a
// message is escaped by the template for its position.
func TestAngleBracketsAreTextOutsideRichMessages(t *testing.T) {
	pieces, err := ParseText("a < b", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := literalText(pieces); got != "a < b" {
		t.Errorf("literal = %q, want the text unchanged", got)
	}
}

func TestNestedHoleIsRejected(t *testing.T) {
	if _, err := ParseText("<a>x<b>y</b></a>", true); err == nil {
		t.Fatal("a nested hole should be rejected")
	}
}

func TestUnsupportedParameterTypeIsNamed(t *testing.T) {
	_, err := Generate(load(t, map[string]string{"m.yaml": `
at:
  params: ["when time.Time"]
  locales: {ja: "{when}"}
`}, []string{"ja"}, "ja"), "messages")
	if err == nil || !strings.Contains(err.Error(), "time.Time") {
		t.Fatalf("expected the unsupported type to be named, got %v", err)
	}
}
