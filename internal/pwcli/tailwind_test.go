package pwcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTailwindReplacesOutputOnlyAfterSuccess(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(bin, "tailwindcss")
	writeTestFile(t, tool, `#!/bin/sh
input=
output=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -i) input="$2"; shift 2 ;;
    -o) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
if grep -q BROKEN "$input"; then
  echo "invalid CSS" >&2
  exit 1
fi
cp "$input" "$output"
`)
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	input := filepath.Join(root, "assets", "app.css")
	output := filepath.Join(root, "internal", "static", "app.css")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, input, "@import \"tailwindcss\";\nGOOD")
	writeTestFile(t, output, "OLD")
	config := tailwindConfig{
		Enabled: true,
		Input:   "assets/app.css",
		Output:  "internal/static/app.css",
		Minify:  true,
	}
	var stdout, stderr bytes.Buffer
	if err := buildTailwind(context.Background(), root, config, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, output, "@import \"tailwindcss\";\nGOOD")

	writeTestFile(t, input, "@import \"tailwindcss\";\nBROKEN")
	err := buildTailwind(context.Background(), root, config, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "tailwindcss") {
		t.Fatalf("expected Tailwind failure, got %v", err)
	}
	assertFileContent(t, output, "@import \"tailwindcss\";\nGOOD")
	if !strings.Contains(stderr.String(), "tailwind: invalid CSS") {
		t.Fatalf("diagnostic is not prefixed: %q", stderr.String())
	}
}

func TestValidateTailwindRejectsMissingLocalPlugin(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(bin, "tailwindcss")
	writeTestFile(t, tool, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "assets", "app.css"), `@import "tailwindcss";
@plugin "./plugins/daisyui.mjs";
`)
	_, _, err := validateTailwind(root, tailwindConfig{
		Enabled: true,
		Input:   "assets/app.css",
		Output:  "static/app.css",
	})
	if err == nil || !strings.Contains(err.Error(), "daisyui.mjs") {
		t.Fatalf("expected missing-plugin error, got %v", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}
