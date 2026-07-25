package pwcli

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfigTailwind(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), `[project]
name = "fixture"
main = "./cmd/fixture"

[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "internal/static/app.css"
minify = true
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Tailwind.Enabled || !config.Tailwind.Minify ||
		config.Tailwind.Input != "assets/app.css" ||
		config.Tailwind.Output != "internal/static/app.css" {
		t.Fatalf("unexpected Tailwind config: %#v", config.Tailwind)
	}
}

func TestLoadProjectConfigRejectsUnknownKeys(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), `[project]
name = "fixture"
main = "."
server.port = 8080
`)
	_, err := loadProjectConfig(root)
	if err == nil || !strings.Contains(err.Error(), "unknown key project.server.port") {
		t.Fatalf("expected unknown-key error, got %v", err)
	}
}

func TestLoadProjectConfigDefaultsEnabledTailwindPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"), `[project]
name = "fixture"
main = "."

[assets.tailwind]
enabled = true
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Tailwind.Input != defaultTailwindInput || config.Tailwind.Output != defaultTailwindOutput {
		t.Fatalf("unexpected Tailwind defaults: %#v", config.Tailwind)
	}
}

func TestScaffoldFilesWithTailwind(t *testing.T) {
	files := scaffoldFilesWithTailwind("fixture", true)
	for _, name := range []string{
		"assets/app.css",
		"public/generated/app.css",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing Tailwind scaffold file %s", name)
		}
	}
	for name, want := range map[string]string{
		"popcornwave.toml":      `output = "public/generated/app.css"`,
		"devbox.json":           "tailwindcss_4@4.1.18",
		"handlers/home.pw.html": `href="/public/generated/app.css"`,
	} {
		if !strings.Contains(files[name], want) {
			t.Errorf("%s does not contain %q:\n%s", name, want, files[name])
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "handlers/index.go", files["handlers/index.go"], parser.AllErrors); err != nil {
		t.Fatalf("Tailwind handler scaffold is invalid Go: %v\n%s", err, files["handlers/index.go"])
	}
	plain := scaffoldFiles("fixture")
	if strings.Contains(plain["devbox.json"], "tailwindcss") {
		t.Fatal("plain scaffold unexpectedly installs Tailwind")
	}
	if _, ok := plain["assets/app.css"]; ok {
		t.Fatal("plain scaffold unexpectedly contains a Tailwind entry")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "handlers/index.go", plain["handlers/index.go"], parser.AllErrors); err != nil {
		t.Fatalf("plain handler scaffold is invalid Go: %v\n%s", err, plain["handlers/index.go"])
	}
}

func TestMainInitUsageMentionsTailwind(t *testing.T) {
	var output strings.Builder
	code := Main([]string{"init"}, &output, &output)
	if code != 1 || !strings.Contains(output.String(), "[--tailwind]") {
		t.Fatalf("unexpected result code=%d output=%q", code, output.String())
	}
}

func TestScaffoldTailwindUsesGeneratedPublicDirectory(t *testing.T) {
	files := scaffoldFilesWithTailwind("fixture", true)
	if _, ok := files["internal/static/static.go"]; ok {
		t.Fatal("Tailwind scaffold unexpectedly creates its former private static package")
	}
	if _, ok := files["public/generated/app.css"]; !ok {
		t.Fatal("Tailwind scaffold is missing its generated public stylesheet")
	}
}

func TestTailwindWatchPathsSkipPublicOutput(t *testing.T) {
	root := t.TempDir()
	config := tailwindConfig{
		Enabled: true,
		Input:   "assets/app.css",
		Output:  "public/generated/app.css",
	}
	if paths := tailwindWatchPaths(root, config, false); len(paths) != 0 {
		t.Fatalf("public Tailwind output should not restart the application: %v", paths)
	}
	paths := tailwindWatchPaths(root, config, true)
	if len(paths) != 1 || paths[0] != filepath.Join(root, "assets", "app.css") {
		t.Fatalf("watch recovery should include only the Tailwind input: %v", paths)
	}

	config.Output = "internal/static/app.css"
	paths = tailwindWatchPaths(root, config, false)
	if len(paths) != 1 || paths[0] != filepath.Join(root, "internal", "static", "app.css") {
		t.Fatalf("custom output should restart the application: %v", paths)
	}
}

func TestSnapshotWatchFilesIgnoresPublicTreeAndPublicGo(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "app.go"), "package fixture\n")
	writeTestFile(t, filepath.Join(root, "public.go"), "package fixture\n")
	if err := os.MkdirAll(filepath.Join(root, "public", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "public", "generated", "asset.go"), "static content\n")

	state, err := snapshotWatchFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state[filepath.Join(root, "app.go")]; !ok {
		t.Fatal("ordinary Go source is missing from watch state")
	}
	for _, ignored := range []string{
		filepath.Join(root, "public.go"),
		filepath.Join(root, "public", "generated", "asset.go"),
	} {
		if _, ok := state[ignored]; ok {
			t.Errorf("public asset path unexpectedly triggers an application rebuild: %s", ignored)
		}
	}
}
