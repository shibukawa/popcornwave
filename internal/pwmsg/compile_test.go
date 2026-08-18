package pwmsg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGeneratedPackageCompilesAndRenders builds the generated package against
// the real pw, pwruntime and htmlbind, then renders every message shape.
//
// Every other test here matches strings against emitted source, which proves
// the generator wrote what was intended and not that what was intended is valid
// Go against the runtime it calls. This one is the check that catches a renamed
// runtime helper, a table whose element type no longer matches, or a signature
// the emitter and the renderer disagree about.
func TestGeneratedPackageCompilesAndRenders(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a module in a temporary directory")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}

	catalog := load(t, map[string]string{"shop.yaml": `
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
      one: "{n} item in your cart"
      other: "{n} items in your cart"

agree:
  rich: true
  locales:
    ja: "利用規約に同意の上、<a>開始</a>してください"
    en: "Please <a>get started</a> after agreeing to the terms"
`}, []string{"ja", "en"}, "ja")

	if diagnostics := Validate(catalog, Error); len(diagnostics) != 0 {
		t.Fatalf("catalog should be clean: %v", diagnostics)
	}
	generated := generate(t, catalog)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "messages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "messages", "messages_pw_gen.go"), generated.Source, 0o644); err != nil {
		t.Fatal(err)
	}

	root := moduleRoot(t)
	goMod := fmt.Sprintf(`module msgcompile

go %s

require (
	github.com/shibukawa/popcornweb v0.0.0
	github.com/shibukawa/tinybind-go v0.5.13
)

replace github.com/shibukawa/popcornweb => %s
`, goVersion(t, root), root)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	// The harness renders one message of every shape and prints the results, so
	// a compile failure and a wrong rendering both surface here.
	main := `package main

import (
	"fmt"

	"msgcompile/messages"
)

func main() {
	for _, loc := range []struct {
		name string
		v    interface{ Tag() string }
	}{{"ja", messages.JA}, {"en", messages.EN}} {
		_ = loc
	}
	fmt.Println(messages.ShopTitle(messages.JA))
	fmt.Println(messages.ShopTitle(messages.EN))
	fmt.Println(messages.ShopGreeting(messages.EN, "Ada"))
	fmt.Println(messages.ShopItemCount(messages.EN, 1))
	fmt.Println(messages.ShopItemCount(messages.EN, 3))
	fmt.Println(messages.ShopItemCount(messages.JA, 3))
	for _, segment := range messages.ShopAgree(messages.EN) {
		fmt.Printf("[%s|%s]", segment.Hole, segment.Text)
	}
	fmt.Println()
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, error) {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	out, err := run("run", ".")
	if err != nil {
		t.Fatalf("generated package does not build or run: %v\n%s\n--- generated ---\n%s", err, out, generated.Source)
	}

	want := []string{
		"お店",
		"Shop",
		"Welcome, Ada!",
		"1 item in your cart",
		"3 items in your cart",
		// Japanese has one plural form, so the same call at a different count
		// renders the same shape. That is the property the per-locale category
		// list exists for.
		"カートに3件",
		// The hole moved to the middle of the English sentence while the
		// template that binds it is unchanged.
		"[|Please ][a|get started][| after agreeing to the terms]",
	}
	for _, line := range want {
		if !strings.Contains(out, line) {
			t.Errorf("output is missing %q; got:\n%s", line, out)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func goVersion(t *testing.T, root string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	t.Fatal("no go directive in the module's go.mod")
	return ""
}
