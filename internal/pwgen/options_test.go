package pwgen

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestOptionsAreValid(t *testing.T) {
	options, err := Options(sqlbind.DialectPostgreSQL)
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
	options, err := Options(sqlbind.DialectPostgreSQL)
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
		byKind[artifact.Kind] += string(artifact.Content)
	}
	assertContains(t, "binding", byKind[generator.ArtifactBinding],
		"RegisterBind[request]", "RegisterWrite[response]",
		// The write-status response goes through a registered writer too,
		// which is what pw.WriteStatus serializes with at runtime.
		"RegisterWrite[created]",
	)
	assertContains(t, "configbind", byKind[generator.ArtifactConfigBind],
		"Register[AppConfig]",
		"RegisterSubCommand[ImportCommand]",
		`Name:      "import"`,
		`Help:      "import data"`,
	)
	// The write-status call contributes its static status: the created schema
	// answers under a 201, and under no other success code.
	assertContains(t, "openapi", byKind[generator.ArtifactOpenAPI], `/items/{id}`, `\"get\"`, `\"post\"`, `\"201\"`)
	if strings.Contains(byKind[generator.ArtifactOpenAPI], `\"200\":{\"content\":{\"application/json\":{\"schema\":{\"$ref\":\"#/components/schemas/created\"`) {
		t.Fatal("the write-status operation also documents a 200 for its response schema")
	}
}

func assertContains(t *testing.T, name, content string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(content, fragment) {
			t.Fatalf("%s does not contain %q:\n%s", name, fragment, content)
		}
	}
}
