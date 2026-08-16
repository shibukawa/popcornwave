package pwgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwmsg"
	templates "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const messagePackage = "example.com/app/messages"

// compileTemplate runs one .pw.html through the real upstream compiler with the
// bindings and symbol table this package supplies.
//
// It is the check that the two halves agree. The generator tests prove the
// catalog produces the symbols it says it does, and the upstream tests prove a
// reference resolves against a table; only this proves the table this framework
// builds is the one a template written against this framework resolves through.
func compileTemplate(t *testing.T, catalog map[string]string, source string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range catalog {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := pwmsg.Load(dir, []string{"ja", "en"}, "ja")
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := pwmsg.Validate(loaded, pwmsg.Error); len(diagnostics) != 0 {
		t.Fatalf("catalog is not clean: %v", diagnostics)
	}
	generated, err := pwmsg.Generate(loaded, "messages")
	if err != nil {
		t.Fatal(err)
	}

	options := templates.GenerateOptions{
		ImplicitBindings:      MessageBindings(),
		MessageContextBinding: MessageLocaleBinding,
		Messages:              map[string]templates.MessageSymbol{},
	}
	for id, symbol := range generated.Symbols {
		options.Messages[id] = templates.MessageSymbol{
			Package: messagePackage,
			Name:    symbol.Name,
			Params:  symbol.Params,
		}
	}

	out, err := templates.Generate("page.pw.html", []byte(source), options)
	if err != nil {
		t.Fatalf("template does not compile: %v\n--- source ---\n%s", err, source)
	}
	return string(out)
}

var shopCatalog = map[string]string{"shop.yaml": `
title:
  locales:
    ja: "お店"
    en: "Shop"

greeting:
  params: ["name string"]
  locales:
    ja: "ようこそ、{name}さん"
    en: "Welcome, {name}!"

item-count:
  params: ["n int"]
  plural: n
  locales:
    ja: "カートに{n}件"
    en:
      one: "{n} item"
      other: "{n} items"
`}

func TestReferenceResolvesThroughTheSuppliedTable(t *testing.T) {
	out := compileTemplate(t, shopCatalog, `messages shop

component Page(): html {
  <h1>{t title}</h1>
}
`)
	if !strings.Contains(out, "ShopTitle(") {
		t.Errorf("the reference did not lower to the generated symbol:\n%s", out)
	}
	if !strings.Contains(out, messagePackage) {
		t.Errorf("the message package was not imported:\n%s", out)
	}
}

// A hyphenated ID is the reason the mapping is a table rather than a naming
// convention: the reference is legal and the symbol name cannot be derived from
// it by the compiler.
func TestHyphenatedIDResolvesToItsSymbol(t *testing.T) {
	out := compileTemplate(t, shopCatalog, `messages shop

component Page(n: int): html {
  <p>{t item-count, n: n}</p>
}
`)
	if !strings.Contains(out, "ShopItemCount(") {
		t.Errorf("a hyphenated id did not resolve:\n%s", out)
	}
}

// The locale reaches a message through the declared binding rather than through
// the render context, which is what makes a reference visible to the cache-key
// walk upstream.
func TestMessageCallTakesTheLocaleBinding(t *testing.T) {
	out := compileTemplate(t, shopCatalog, `messages shop

component Page(name: string): html {
  <p>{t greeting, name: name}</p>
}
`)
	if !strings.Contains(out, "pwruntime.MessageLocale(") {
		t.Errorf("the message call did not read the locale binding:\n%s", out)
	}
}

func TestUnknownIDFailsGeneration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shop.yaml"), []byte("title:\n  locales: {ja: \"x\", en: \"y\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, _ := pwmsg.Load(dir, []string{"ja", "en"}, "ja")
	generated, _ := pwmsg.Generate(loaded, "messages")

	options := templates.GenerateOptions{
		ImplicitBindings:      MessageBindings(),
		MessageContextBinding: MessageLocaleBinding,
		Messages:              map[string]templates.MessageSymbol{},
	}
	for id, symbol := range generated.Symbols {
		options.Messages[id] = templates.MessageSymbol{Package: messagePackage, Name: symbol.Name, Params: symbol.Params}
	}

	_, err := templates.Generate("page.pw.html", []byte(`messages shop

component Page(): html {
  <h1>{t nonexistent}</h1>
}
`), options)
	if err == nil {
		t.Fatal("a reference to an undeclared message should fail generation")
	}
	if !strings.Contains(err.Error(), "shop.nonexistent") {
		t.Errorf("the error should name the resolved id, got: %v", err)
	}
}

// The path-segment binding is the one admitted into a URL attribute despite not
// being url-typed. That it compiles there at all is the amendment upstream had
// to make, so it is worth asserting rather than assuming.
func TestLangSegmentCompilesInsideAURLAttribute(t *testing.T) {
	out := compileTemplate(t, shopCatalog, `messages shop

component Page(): html {
  <a href="/{lang}/about">{t title}</a>
}
`)
	if !strings.Contains(out, "pwruntime.LangSegment(") {
		t.Errorf("the segment binding was not read:\n%s", out)
	}
	if !strings.Contains(out, "URLPathSegment") {
		t.Errorf("the segment did not go through the collapsing helper:\n%s", out)
	}
}

func TestLangTagCompilesInAnOrdinaryAttribute(t *testing.T) {
	out := compileTemplate(t, shopCatalog, `messages shop

component Page(): html {
  <html lang="{langtag}"><body>{t title}</body></html>
}
`)
	if !strings.Contains(out, "pwruntime.LangTag(") {
		t.Errorf("the tag binding was not read:\n%s", out)
	}
}

// The typed binding must not be writable into markup: there is no escaping rule
// for a type upstream has never seen, and a value that reached a page would be
// whatever Go's default formatting made of it.
func TestTypedLocaleBindingCannotBeWrittenIntoMarkup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shop.yaml"), []byte("title:\n  locales: {ja: \"x\", en: \"y\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, _ := pwmsg.Load(dir, []string{"ja", "en"}, "ja")
	generated, _ := pwmsg.Generate(loaded, "messages")
	options := templates.GenerateOptions{
		ImplicitBindings:      MessageBindings(),
		MessageContextBinding: MessageLocaleBinding,
		Messages:              map[string]templates.MessageSymbol{},
	}
	for id, symbol := range generated.Symbols {
		options.Messages[id] = templates.MessageSymbol{Package: messagePackage, Name: symbol.Name, Params: symbol.Params}
	}

	_, err := templates.Generate("page.pw.html", []byte(`messages shop

component Page(): html {
  <p>{`+MessageLocaleBinding+`}</p>
}
`), options)
	if err == nil {
		t.Fatal("writing the typed locale binding into markup should fail generation")
	}
}

// A project that declares no catalog still compiles: the bindings are
// unconditional, and a template that reads none of them generates as before.
func TestBindingsAreFreeWhenUnused(t *testing.T) {
	options := templates.GenerateOptions{
		ImplicitBindings:      MessageBindings(),
		MessageContextBinding: MessageLocaleBinding,
	}
	source := []byte(`component Page(title: string): html {
  <h1>{title}</h1>
}
`)
	withBindings, err := templates.Generate("page.pw.html", source, options)
	if err != nil {
		t.Fatal(err)
	}
	without, err := templates.Generate("page.pw.html", source, templates.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(withBindings) != string(without) {
		t.Errorf("declaring bindings changed the output of a template that reads none:\n--- with ---\n%s\n--- without ---\n%s", withBindings, without)
	}
}

// A parameter taking a binding's name must be reported, or the framework value
// an author thinks they are reading is silently whatever the parameter holds.
func TestParameterShadowingABindingIsReported(t *testing.T) {
	options := templates.GenerateOptions{
		ImplicitBindings:      MessageBindings(),
		MessageContextBinding: MessageLocaleBinding,
	}
	_, err := templates.Generate("page.pw.html", []byte(`component Page(`+LangTagBinding+`: string): html {
  <p>{`+LangTagBinding+`}</p>
}
`), options)
	if err == nil {
		t.Fatalf("a parameter named %q should collide with the declared binding", LangTagBinding)
	}
}

func TestApplyMessagesLeavesNoTableForAnEmptyCatalog(t *testing.T) {
	generatorOptions, err := Options("postgresql")
	if err != nil {
		t.Fatal(err)
	}
	ApplyMessages(&generatorOptions, nil, messagePackage)
	if generatorOptions.Messages != nil {
		t.Error("an empty catalog should leave no symbol table, so any reference fails generation")
	}
	if generatorOptions.MessageContextBinding != MessageLocaleBinding {
		t.Error("the context binding should be named even with no catalog")
	}
	if len(generatorOptions.ImplicitBindings) != 3 {
		t.Errorf("bindings = %d, want the three of data:locale-bindings", len(generatorOptions.ImplicitBindings))
	}
}
