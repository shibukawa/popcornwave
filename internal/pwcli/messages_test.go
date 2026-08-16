package pwcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeI18nFixture creates a project declaring two locales, a catalog, and one
// template that references a message and writes the locale into a link.
func writeI18nFixture(t *testing.T, catalog string) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join("cmd", "fixture"),
		"messages",
		"templates",
	} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\n"+
			"handlers = []\ntemplates = [\"templates\"]\nqueries = []\nconfig = []\n\n"+
			"[i18n]\nlocales = [\"ja\", \"en\"]\ndefault_locale = \"ja\"\n"+
			"path_routes = [\"/\"]\n\n[i18n.label]\nja = \"日本語\"\nen = \"English\"\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "messages", "shop.yaml"), catalog)
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

messages shop

export component Card(): html {
<section>
  <h1>{t title}</h1>
  <a href="/{lang}/about">{t title}</a>
</section>
}
`)
	return root
}

const shopCatalogSource = `
title:
  locales:
    ja: "お店"
    en: "Shop"
`

// The whole chain in one run: the catalog is read, the message package is
// written, its symbols reach the template compiler, and the reference lowers to
// a call on the generated function.
func TestGenerateWritesTheMessagePackageAndResolvesAReference(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	generateIn(t, root)

	generated := readTestFile(t, filepath.Join(root, "messages", messagesGeneratedName))
	if !strings.Contains(generated, "package messages") {
		t.Errorf("message package has the wrong package clause:\n%s", generated)
	}
	if !strings.Contains(generated, "func ShopTitle(loc pw.Locale) string") {
		t.Errorf("message function is missing:\n%s", generated)
	}
	if !strings.Contains(generated, `RegisterLocales([]string{"ja", "en"}, "ja")`) {
		t.Errorf("the declared locales were not registered:\n%s", generated)
	}

	compiled := readTestFile(t, filepath.Join(root, "templates", "card_pw_gen.go"))
	if !strings.Contains(compiled, "ShopTitle(") {
		t.Errorf("the reference did not resolve to the generated symbol:\n%s", compiled)
	}
	if !strings.Contains(compiled, "example.test/fixture/messages") {
		t.Errorf("the message package was not imported by its module path:\n%s", compiled)
	}
	if !strings.Contains(compiled, "pwruntime.LangSegment(") {
		t.Errorf("the locale segment binding was not read:\n%s", compiled)
	}
}

// Generation is stable, so a second run writes nothing and --check passes.
func TestGenerateIsStableWithACatalog(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	generateIn(t, root)
	if output := generateIn(t, root, "--check"); strings.Contains(output, messagesGeneratedName) {
		t.Errorf("a second run reports the message package as stale:\n%s", output)
	}
}

// A catalog error stops generation and names the message, rather than emitting
// a package the template would then fail against for a second reason.
func TestGenerateFailsOnACatalogError(t *testing.T) {
	root := writeI18nFixture(t, `
title:
  params: ["name string"]
  locales:
    ja: "お店"
    en: "Shop"
`)
	output, err := generateExpectingFailure(t, root)
	if err == nil {
		t.Fatalf("generation should have failed:\n%s", output)
	}
	if !strings.Contains(output, "never uses declared parameter") {
		t.Errorf("the report does not name the problem:\n%s", output)
	}
	if !strings.Contains(output, "shop.title") {
		t.Errorf("the report does not name the message:\n%s", output)
	}
}

// A missing translation follows the configured severity, so a project mid
// translation keeps building and one that must ship complete does not.
func TestMissingTranslationSeverityIsRead(t *testing.T) {
	incomplete := `
title:
  locales:
    ja: "お店"
`
	root := writeI18nFixture(t, incomplete)
	if output, err := generateExpectingFailure(t, root); err == nil {
		t.Fatalf("the default severity should stop the build:\n%s", output)
	}

	root = writeI18nFixture(t, incomplete)
	config := readTestFile(t, filepath.Join(root, "popcornwave.toml"))
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		strings.Replace(config, "default_locale = \"ja\"", "default_locale = \"ja\"\nmissing = \"warn\"", 1))
	output := generateIn(t, root)
	if !strings.Contains(output, "supplies no translation") {
		t.Errorf("the warning was not reported:\n%s", output)
	}
	// Fallback is flattened at generation, so the English row carries the
	// Japanese text rather than an empty string a render would have to answer.
	generated := readTestFile(t, filepath.Join(root, "messages", messagesGeneratedName))
	if strings.Count(generated, `"お店"`) != 2 {
		t.Errorf("the missing locale was not filled from the fallback chain:\n%s", generated)
	}
}

// A project declaring no locale is unchanged: no message package, and the
// bindings a template never reads cost it nothing.
func TestProjectWithoutLocalesGeneratesNoMessagePackage(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	config := readTestFile(t, filepath.Join(root, "popcornwave.toml"))
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		config[:strings.Index(config, "[i18n]")])
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

export component Card(): html {
<h1>plain</h1>
}
`)
	generateIn(t, root)

	if _, err := os.Stat(filepath.Join(root, "messages", messagesGeneratedName)); !os.IsNotExist(err) {
		t.Error("a project declaring no locale should generate no message package")
	}
}

// A reference to a message the catalog does not declare fails at generation,
// which is the check the whole typed shape exists for.
func TestUndeclaredReferenceFailsGeneration(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

messages shop

export component Card(): html {
<h1>{t nonexistent}</h1>
}
`)
	output, err := generateExpectingFailure(t, root)
	if err == nil {
		t.Fatalf("an undeclared reference should fail:\n%s", output)
	}
	if !strings.Contains(output+err.Error(), "shop.nonexistent") {
		t.Errorf("the failure does not name the resolved id: %v\n%s", err, output)
	}
}

// generateExpectingFailure runs generation and returns its output and error
// rather than failing the test, so a case asserting a refusal can read both.
// The diagnostics a refusal prints are part of what is being asserted, and
// generateIn discards them by calling t.Fatalf.
func generateExpectingFailure(t *testing.T, root string) (string, error) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var output strings.Builder
	err = runGenerate(context.Background(), nil, &output)
	return output.String(), err
}

// pw i18n asks the question a build never does: whether a declared message is
// still reached by anything.
func TestI18nCheckReportsUnusedAndUndeclared(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource+`
orphan:
  locales:
    ja: "誰も使わない"
    en: "nobody uses this"
`)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var output strings.Builder
	if err := runI18n([]string{"check"}, &output); err != nil {
		t.Fatalf("i18n check: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), `"shop.orphan" is declared and referenced by no template`) {
		t.Errorf("the unused message was not reported:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "2 messages, 2 locales, 1 referenced") {
		t.Errorf("the summary is wrong:\n%s", output.String())
	}
}

// A reference no catalog declares is an error here as well as at generation, so
// a translator running this sees the same set a build would refuse.
func TestI18nCheckReportsAnUndeclaredReference(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

messages shop

export component Card(): html {
<h1>{t nonexistent}</h1>
}
`)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var output strings.Builder
	err = runI18n([]string{"check"}, &output)
	if err == nil {
		t.Fatalf("an undeclared reference should fail the check:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "shop.nonexistent") {
		t.Errorf("the report does not name the message:\n%s", output.String())
	}
}

func inProject(t *testing.T, root string, fn func()) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	fn()
}

// Extraction turns marked text into a reference, a catalog entry, and a scope
// declaration, and the file it rewrote still parses.
func TestI18nExtractRewritesTemplatesAndWritesEntries(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

export component Card(): html {
<section>
  <h1 i18n>Welcome back</h1>
  <input placeholder="Your name" i18n="placeholder">
</section>
}
`)
	var output strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"extract", "--scope=card"}, &output); err != nil {
			t.Fatalf("extract: %v\n%s", err, output.String())
		}
	})

	rewritten := readTestFile(t, filepath.Join(root, "templates", "card.pw.html"))
	if !strings.Contains(rewritten, "{t welcome-back}") {
		t.Errorf("element text was not replaced by a reference:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "{t your-name}") {
		t.Errorf("attribute value was not replaced:\n%s", rewritten)
	}
	if strings.Contains(rewritten, "i18n") {
		t.Errorf("the extraction mark should not survive the rewrite:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "messages card") {
		t.Errorf("the scope declaration was not added:\n%s", rewritten)
	}

	catalog := readTestFile(t, filepath.Join(root, "messages", "card.yaml"))
	for _, want := range []string{"welcome-back:", `snapshot: "Welcome back"`, `ja: "Welcome back"`, `en: ""`} {
		if !strings.Contains(catalog, want) {
			t.Errorf("catalog is missing %q:\n%s", want, catalog)
		}
	}

	// The whole point of rewriting rather than reporting: the result compiles.
	inProject(t, root, func() {
		var second strings.Builder
		if err := runGenerate(context.Background(), nil, &second); err != nil {
			t.Fatalf("the extracted template does not generate: %v\n%s", err, second.String())
		}
	})
}

// Rich text is declined rather than guessed at, because a hole name invented by
// a tool is one no translator can check.
func TestI18nExtractDeclinesRichText(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

export component Card(): html {
<p i18n>Please <a href="/start">get started</a> first</p>
}
`)
	var output strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"extract"}, &output); err != nil {
			t.Fatalf("extract: %v\n%s", err, output.String())
		}
	})
	if !strings.Contains(output.String(), "rich text") {
		t.Errorf("the decline was not explained:\n%s", output.String())
	}
	if strings.Contains(readTestFile(t, filepath.Join(root, "templates", "card.pw.html")), "{t ") {
		t.Error("a declined mark should leave the template untouched")
	}
}

// A text no slug can be derived from still gets an ID, and says so.
func TestI18nExtractNamesWhatItCouldNotSlug(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	writeTestFile(t, filepath.Join(root, "templates", "card.pw.html"), `package templates

export component Card(): html {
<h1 i18n>新規登録</h1>
}
`)
	var output strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"extract", "--scope=card"}, &output); err != nil {
			t.Fatalf("extract: %v\n%s", err, output.String())
		}
	})
	if !strings.Contains(output.String(), "no slug could be derived") {
		t.Errorf("the fallback was not reported:\n%s", output.String())
	}
	if !strings.Contains(readTestFile(t, filepath.Join(root, "templates", "card.pw.html")), "{t h1-1}") {
		t.Error("a positional id should have been assigned")
	}
}

func TestI18nRenameCarriesEveryTranslation(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	var output strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"rename", "shop.title", "shop.heading"}, &output); err != nil {
			t.Fatalf("rename: %v\n%s", err, output.String())
		}
	})
	catalog := readTestFile(t, filepath.Join(root, "messages", "shop.yaml"))
	if strings.Contains(catalog, "title:") || !strings.Contains(catalog, "heading:") {
		t.Errorf("the id was not renamed:\n%s", catalog)
	}
	for _, want := range []string{`ja: "お店"`, `en: "Shop"`} {
		if !strings.Contains(catalog, want) {
			t.Errorf("rename dropped %q:\n%s", want, catalog)
		}
	}
}

// The round trip: export a locale, translate it, import it back, and the other
// locale is untouched.
func TestI18nExportImportRoundTrip(t *testing.T) {
	root := writeI18nFixture(t, `
title:
  locales:
    ja: "お店"
    en: ""

item-count:
  params: ["n int"]
  plural: n
  locales:
    ja: "カートに{n}件"
    en:
      one: ""
      other: ""
`)
	var exported strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"export", "en"}, &exported); err != nil {
			t.Fatalf("export: %v\n%s", err, exported.String())
		}
	})
	po := exported.String()
	for _, want := range []string{`msgctxt "shop.title"`, `msgid "お店"`, `msgid_plural`, `msgstr[0] ""`} {
		if !strings.Contains(po, want) {
			t.Errorf("PO is missing %q:\n%s", want, po)
		}
	}

	translated := strings.Replace(po, "msgctxt \"shop.title\"\nmsgid \"お店\"\nmsgstr \"\"",
		"msgctxt \"shop.title\"\nmsgid \"お店\"\nmsgstr \"Shop\"", 1)
	translated = strings.Replace(translated, `msgstr[0] ""`, `msgstr[0] "{n} item"`, 1)
	translated = strings.Replace(translated, `msgstr[1] ""`, `msgstr[1] "{n} items"`, 1)
	poPath := filepath.Join(root, "en.po")
	writeTestFile(t, poPath, translated)

	var output strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"import", "en", poPath}, &output); err != nil {
			t.Fatalf("import: %v\n%s", err, output.String())
		}
	})
	catalog := readTestFile(t, filepath.Join(root, "messages", "shop.yaml"))
	if !strings.Contains(catalog, `en: "Shop"`) {
		t.Errorf("the simple translation was not applied:\n%s", catalog)
	}
	for _, want := range []string{`one: "{n} item"`, `other: "{n} items"`} {
		if !strings.Contains(catalog, want) {
			t.Errorf("the plural forms were not applied:\n%s", catalog)
		}
	}
	// Import writes target translations only. The source locale is the
	// catalog's and a PO file has no business changing it.
	if !strings.Contains(catalog, `ja: "お店"`) {
		t.Errorf("import touched the source locale:\n%s", catalog)
	}

	inProject(t, root, func() {
		var second strings.Builder
		if err := runGenerate(context.Background(), nil, &second); err != nil {
			t.Fatalf("the imported catalog does not generate: %v\n%s", err, second.String())
		}
	})
}

// A PO file naming a message the catalog does not declare is reported and
// skipped, never invented into the catalog.
func TestI18nImportSkipsUndeclaredMessages(t *testing.T) {
	root := writeI18nFixture(t, shopCatalogSource)
	poPath := filepath.Join(root, "en.po")
	writeTestFile(t, poPath, "msgid \"\"\nmsgstr \"\"\n\nmsgctxt \"shop.ghost\"\nmsgid \"x\"\nmsgstr \"y\"\n")

	var output strings.Builder
	inProject(t, root, func() {
		if err := runI18n([]string{"import", "en", poPath}, &output); err != nil {
			t.Fatalf("import: %v\n%s", err, output.String())
		}
	})
	if !strings.Contains(output.String(), "shop.ghost") || !strings.Contains(output.String(), "skipped") {
		t.Errorf("the undeclared message was not reported:\n%s", output.String())
	}
}
