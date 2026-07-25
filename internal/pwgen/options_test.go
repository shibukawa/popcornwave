package pwgen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestOptionsAreValid(t *testing.T) {
	options, err := Options()
	if err != nil {
		t.Fatal(err)
	}
	if !options.SQLContextOnlyAPI || options.SQLExecutorResolver == nil {
		t.Fatal("SQL context-only resolver is not configured")
	}
	if !options.HTMLWriterAPI || options.HTMLTemplatePattern != "*.pw.html" || options.SQLTemplatePattern != "*.pw.sql" {
		t.Fatal("Popcorn Wave template generation profile is not configured")
	}
}

func TestPWCallsDriveTinyBindGeneration(t *testing.T) {
	options, err := Options()
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{
		Dir: filepath.Join("testdata", "wrappers"), Out: output,
		Name: "bindings.go", ConfigBindName: "config.go",
		OpenAPI: true, OpenAPIName: "openapi.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, result.BinderPath, "RegisterBind[request]", "RegisterWrite[response]")
	assertContains(t, result.ConfigBindPath,
		"Register[AppConfig]",
		"RegisterSubCommand[ImportCommand]",
		`Name:      "import"`,
		`Help:      "import data"`,
	)
	assertContains(t, result.OpenAPIPath, `/items/{id}`, `\"get\"`)
}

func assertContains(t *testing.T, path string, fragments ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("%s does not contain %q:\n%s", path, fragment, content)
		}
	}
}
