package pwcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeWidgetPackage creates a component package module on disk: a manifest, a
// go.mod, one Go file, and a migration stream. It is a real module rather than a
// fake so the Go tool resolves it the way it resolves any dependency.
func writeWidgetPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/widget\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(dir, "widget.go"), "package widget\n")
	writeTestFile(t, filepath.Join(dir, "popcornwave.toml"), `[project]
name = "widget"
kind = "package"

[package]
module = "example.com/widget"
summary = "A widget"

[package.migrations]
dir = "migrations"
stem = "widget"
engines = ["sqlite"]

[generate]
handlers = []
templates = []
queries = []
config = []
`)
	writeTestFile(t, filepath.Join(dir, "migrations", "00001_init.sql"),
		"-- +goose Up\ncreate table widget_thing (id integer primary key);\n")
}

// consumingProject writes an application that declares the widget package and
// resolves it through a replace directive, which is how a test reaches a module
// that was never published.
func consumingProject(t *testing.T, root, packageDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.test/app

go 1.26.0

require example.com/widget v0.0.0

replace example.com/widget => `+packageDir+`
`)
	writeTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), `[project]
name = "app"
main = "./cmd/app"

[[packages]]
module = "example.com/widget"

[generate]
handlers = []
templates = []
queries = []
config = []
`)
}

// TestDeclaredPackageResolvesAndLinks is the whole install: one declaration, and
// the generated bootstrap carries the import that links the package. Nothing is
// copied into the project tree.
func TestDeclaredPackageResolvesAndLinks(t *testing.T) {
	packageDir := t.TempDir()
	writeWidgetPackage(t, packageDir)
	root := t.TempDir()
	consumingProject(t, root, packageDir)

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resolved, err := resolvePackages(ctx, root, config.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 || resolved[0].Module != "example.com/widget" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved[0].Manifest.Migrations.Stem != "widget" {
		t.Fatalf("manifest = %#v", resolved[0].Manifest)
	}

	changes, err := planBootstrapLink(root, config, resolved, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %#v", changes)
	}
	if source := string(changes[0].source); !strings.Contains(source, `_ "example.com/widget"`) {
		t.Fatalf("bootstrap does not link the package:\n%s", source)
	}
	// The install writes nothing into the project beyond the generated
	// bootstrap: no migration copy, no configuration section.
	if entries, err := os.ReadDir(root); err == nil {
		for _, entry := range entries {
			if entry.Name() == "migrations" {
				t.Fatal("the install copied a migration directory into the project")
			}
		}
	}
}

// TestDeclaredPackageStreamIsFoundInTheModule proves pw reaches the package's
// migrations without an application binary, reading the module directory rather
// than another process's embedded copy.
func TestDeclaredPackageStreamIsFoundInTheModule(t *testing.T) {
	packageDir := t.TempDir()
	writeWidgetPackage(t, packageDir)
	root := t.TempDir()
	consumingProject(t, root, packageDir)

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resolved, err := resolvePackages(ctx, root, config.Packages)
	if err != nil {
		t.Fatal(err)
	}
	streams, err := packageStreams(ctx, root, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Stem != "widget" {
		t.Fatalf("streams = %#v", streams)
	}
	if _, err := streams[0].Sources.Open("00001_init.sql"); err != nil {
		t.Fatalf("the stream does not carry its migration: %v", err)
	}
}

// TestUndeclaredModuleIsAnOrdinaryDependency covers the other direction: a
// module carrying a package section that the project never declared contributes
// nothing, because the declaration is what links it.
func TestUndeclaredModuleIsAnOrdinaryDependency(t *testing.T) {
	packageDir := t.TempDir()
	writeWidgetPackage(t, packageDir)
	root := t.TempDir()
	consumingProject(t, root, packageDir)
	// Remove the declaration, keeping the go.mod requirement.
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), `[project]
name = "app"
main = "./cmd/app"

[generate]
handlers = []
templates = []
queries = []
config = []
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Packages) != 0 {
		t.Fatalf("packages = %#v", config.Packages)
	}
	resolved, err := resolvePackages(context.Background(), root, config.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 0 {
		t.Fatalf("an undeclared module was resolved: %#v", resolved)
	}
}

func TestDeclaredPackageMissingFromTheModuleGraphIsNamed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/app\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), `[project]
name = "app"
main = "./cmd/app"

[[packages]]
module = "example.com/absent"

[generate]
handlers = []
templates = []
queries = []
config = []
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolvePackages(context.Background(), root, config.Packages)
	if err == nil || !strings.Contains(err.Error(), "example.com/absent") {
		t.Fatalf("err = %v, want the declared module named", err)
	}
}

func TestPackageEngineMismatchIsRefused(t *testing.T) {
	// A package installed into a project whose engine it never wrote schema for
	// would fail at the first migration; naming it at the declaration is the
	// whole point of requires.engines.
	config := projectConfig{Database: "postgres"}
	resolved := []resolvedPackage{{
		Module:   "example.com/widget",
		Manifest: packageManifest{Module: "example.com/widget", Requires: packageRequires{Engines: []string{"sqlite"}}},
	}}
	err := checkPackageCompatibility(config, resolved, nil)
	if err == nil || !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("err = %v, want the project engine named", err)
	}
}

func TestPackageSharedStemIsRefusedAtProjectLevel(t *testing.T) {
	resolved := []resolvedPackage{
		{Module: "example.com/one", Manifest: packageManifest{Module: "example.com/one", Migrations: packageMigrations{Dir: "migrations", Stem: "shared"}}},
		{Module: "example.com/two", Manifest: packageManifest{Module: "example.com/two", Migrations: packageMigrations{Dir: "migrations", Stem: "shared"}}},
	}
	err := checkPackageCompatibility(projectConfig{Database: "sqlite"}, resolved, nil)
	if err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("err = %v, want the shared stem refused", err)
	}
}

func TestIsModulePath(t *testing.T) {
	// A module path carries a dot in its first element, which is what separates
	// it from every built-in capability name.
	for _, arg := range []string{"example.com/widget", "github.com/o/r", "gopkg.in/x.v1"} {
		if !isModulePath(arg) {
			t.Errorf("isModulePath(%q) = false", arg)
		}
	}
	for _, arg := range []string{"database", "auth", "tailwind", "redis-valkey", "dynamo"} {
		if isModulePath(arg) {
			t.Errorf("isModulePath(%q) = true", arg)
		}
	}
}

// TestAddPackageWritesOnlyTheDeclaration is the ergonomic claim under test:
// installing writes one entry and touches nothing else in the project tree.
func TestAddPackageWritesOnlyTheDeclaration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n")
	original := `[project]
name = "app"
main = "./cmd/app"

# an operator comment that must survive the edit

[generate]
handlers = []
templates = []
queries = []
config = []
`
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), original)
	if err := appendPackageDeclaration(root, "example.com/widget"); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(filepath.Join(root, "popcornwave.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), `module = "example.com/widget"`) {
		t.Fatalf("declaration missing:\n%s", updated)
	}
	// The edit appends, so comments and hand tuned values are untouched.
	if !strings.HasPrefix(string(updated), original) {
		t.Fatalf("the existing document was rewritten:\n%s", updated)
	}
}
