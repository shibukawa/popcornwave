package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestPreparePublicAssets(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "public.go"), "package fixture\n")
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "public", "app.css"), "body { color: red }\n")
	writeTestFile(t, filepath.Join(root, "public", "image.png"), "png")
	writeTestFile(t, filepath.Join(root, "public", "stale.txt.zstd"), "stale")

	if err := preparePublicAssets(root); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, "public", "app.css.zstd")
	encoded, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	decoded, err := decoder.DecodeAll(encoded, nil)
	if err != nil || string(decoded) != "body { color: red }\n" {
		t.Fatalf("decoded = %q, %v", decoded, err)
	}
	if _, err := os.Stat(filepath.Join(root, "public", "image.png.zstd")); !os.IsNotExist(err) {
		t.Fatalf("binary sidecar exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "public", "stale.txt.zstd")); !os.IsNotExist(err) {
		t.Fatalf("stale sidecar exists: %v", err)
	}
}

func TestScaffoldIncludesPublicEmbed(t *testing.T) {
	files := scaffoldFiles("fixture")
	for _, name := range []string{"public.go", "public/.keep"} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	for _, fragment := range []string{"pw.WithPublicFS", "publicassets.PublicFS()"} {
		if strings.Contains(files["cmd/fixture/main.go"], fragment) {
			t.Errorf("main.go unexpectedly contains %q", fragment)
		}
	}
	for _, fragment := range []string{
		`"github.com/shibukawa/popcornwave/middlewares"`,
		"func init()",
		"middlewares.RegisterPublicFS(PublicFS())",
	} {
		if !strings.Contains(files["public.go"], fragment) {
			t.Errorf("public.go missing %q", fragment)
		}
	}
}
