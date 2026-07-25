package pwgen

import (
	"context"
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
	if options.HTMLTemplatePattern != "*.pw.html" || options.SQLTemplatePattern != "*.pw.sql" {
		t.Fatal("Popcorn Wave template generation profile is not configured")
	}
}

func TestPWCallsDriveTinyBindGeneration(t *testing.T) {
	options, err := Options()
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := generator.New(options).GenerateArtifacts(context.Background(), generator.GenerateRequest{
		Dir: filepath.Join("testdata", "wrappers"), OpenAPI: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[generator.ArtifactKind]string{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] += string(artifact.GoSource)
	}
	assertContains(t, "binding", byKind[generator.ArtifactBinding], "RegisterBind[request]", "RegisterWrite[response]")
	assertContains(t, "configbind", byKind[generator.ArtifactConfigBind],
		"Register[AppConfig]",
		"RegisterSubCommand[ImportCommand]",
		`Name:      "import"`,
		`Help:      "import data"`,
	)
	assertContains(t, "openapi", byKind[generator.ArtifactOpenAPI], `/items/{id}`, `\"get\"`)
}

func assertContains(t *testing.T, name, content string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(content, fragment) {
			t.Fatalf("%s does not contain %q:\n%s", name, fragment, content)
		}
	}
}
