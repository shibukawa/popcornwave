package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pw"
)

func TestGenerateConfigScaffolds(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{format: "toml", want: "[server]"},
		{format: "env", want: "PORT=8080"},
	}
	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			var output bytes.Buffer
			err := writeConfigScaffold(test.format, &output)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), test.want) {
				t.Fatalf("output = %q, want fragment %q", output.String(), test.want)
			}
		})
	}
}

func TestGenerateConfigRejectsUnknownFormat(t *testing.T) {
	err := writeConfigScaffold("json", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected format error")
	}
}

func TestAssembledOpenAPIUsesApplicationInfo(t *testing.T) {
	if err := configureOpenAPI(); err != nil {
		t.Fatal(err)
	}
	jsonDocument, yamlDocument, err := pw.AssembleOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{string(jsonDocument), string(yamlDocument)} {
		if !strings.Contains(document, "Popcorn Wave Example API") || !strings.Contains(document, "1.0.0") ||
			!strings.Contains(document, "/echo") || !strings.Contains(document, "/openapi.yaml") {
			t.Fatalf("assembled OpenAPI document is incomplete: %s", document)
		}
	}
}
