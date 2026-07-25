package pwcli

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
)

func TestPlanDirectoryGeneratesV015Artifacts(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "home.pw.html"), `package fixture

export component Home(name: string): html {
<h1>Hello, {name}</h1>
}
`)
	writeTestFile(t, filepath.Join(directory, "users.pw.sql"), `package fixture

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`)

	options, err := pwgen.Options()
	if err != nil {
		t.Fatal(err)
	}
	runner := generator.New(options)
	changes, err := planDirectory(context.Background(), runner, directory)
	if err != nil {
		t.Fatal(err)
	}
	byName := changesByBase(changes)
	for _, name := range []string{"home_pw_gen.go", "users_pw_gen.go", "tinybind_shared_pw_gen.go"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("missing generated artifact %s; got %v", name, mapKeys(byName))
		}
	}
	home := string(byName["home_pw_gen.go"].source)
	if !strings.Contains(home, "func Home(params HomeParams) htmlbind.Fragment") {
		t.Errorf("HTML artifact does not use Fragment API:\n%s", home)
	}
	sql := string(byName["users_pw_gen.go"].source)
	if !strings.Contains(sql, "func FindUser(ctx context.Context, id int) (User, error)") {
		t.Errorf("SQL artifact does not use context-only API:\n%s", sql)
	}
	if strings.Contains(sql, "FindUserContext") ||
		strings.Contains(sql, "func FindUser(ctx context.Context, db SQLQuerier") {
		t.Errorf("SQL artifact exposes the legacy executor API:\n%s", sql)
	}

	if err := applyFileChanges(changes); err != nil {
		t.Fatal(err)
	}
	changes, err = planDirectory(context.Background(), runner, directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("generation is not deterministic: %#v", changes)
	}

	stale := filepath.Join(directory, "obsolete_pw_gen.go")
	writeTestFile(t, stale, "package fixture\n")
	changes, err = planDirectory(context.Background(), runner, directory)
	if err != nil {
		t.Fatal(err)
	}
	byName = changesByBase(changes)
	if !byName["obsolete_pw_gen.go"].remove {
		t.Fatalf("stale generated file was not scheduled for removal: %#v", changes)
	}
	if err := applyFileChanges(changes); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale generated file still exists: %v", err)
	}
}

func TestMergeArtifactsProducesOneValidGoFile(t *testing.T) {
	source, err := mergeArtifacts([]generator.Artifact{
		{
			PackageName: "fixture",
			GoSource: []byte(`package fixture

import "fmt"

func first() { fmt.Print("first") }
`),
		},
		{
			PackageName: "fixture",
			GoSource: []byte(`package fixture

import "strings"

func second() { _ = strings.Builder{} }
`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Count(text, "package fixture") != 1 ||
		!strings.Contains(text, `"fmt"`) ||
		!strings.Contains(text, `"strings"`) ||
		!strings.Contains(text, "func first()") ||
		!strings.Contains(text, "func second()") {
		t.Fatalf("unexpected merged source:\n%s", text)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "merged.go", source, parser.AllErrors); err != nil {
		t.Fatalf("merged source is invalid: %v\n%s", err, text)
	}
}

func TestPlanDirectoryRegistersDocumentShell(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "document.pw.html"), `package fixture

export component Document(children: html?): html {
<!doctype html><html><body><slot /></body></html>
}
`)
	options, err := pwgen.Options()
	if err != nil {
		t.Fatal(err)
	}
	changes, err := planDirectory(context.Background(), generator.New(options), directory)
	if err != nil {
		t.Fatal(err)
	}
	document := string(changesByBase(changes)["document_pw_gen.go"].source)
	for _, fragment := range []string{
		`"github.com/shibukawa/popcornwave/pw"`,
		"func init()",
		"pw.RegisterHTMLDocument(BindDocument(DocumentParams{}))",
	} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("document artifact is missing %q:\n%s", fragment, document)
		}
	}
}

func TestPlanBootstrapLinkGeneratesRuntimeRegistrationImports(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "public.go"), "package publicassets\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "templates", "document.pw.html"), `package templates

export component Document(children: html?): html {
<html><body><slot /></body></html>
}
`)
	changes, err := planBootstrapLink(root, "./cmd/fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %#v", changes)
	}
	source := string(changes[0].source)
	for _, expected := range []string{
		`_ "example.test/fixture"`,
		`_ "example.test/fixture/templates"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("link source is missing %q:\n%s", expected, source)
		}
	}
}

func writeTestFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func changesByBase(changes []fileChange) map[string]fileChange {
	out := make(map[string]fileChange, len(changes))
	for _, change := range changes {
		out[filepath.Base(change.path)] = change
	}
	return out
}

func mapKeys(values map[string]fileChange) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
