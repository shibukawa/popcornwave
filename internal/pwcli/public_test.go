package pwcli

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// writeNestedTestFile writes a file and the directories above it.
func writeNestedTestFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, source)
}

// derivedFixture writes the smallest project the derived build accepts: the
// scaffolded embed and an authored tree.
func derivedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "public.go"), "package fixture\n\n//go:embed all:"+derivedPublicDir+"\nvar embeddedPublic embed.FS\n")
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBuildDerivedAssets(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), "body { color: red }\n")
	writeTestFile(t, filepath.Join(root, "public", "image.png"), "png")
	writeTestFile(t, filepath.Join(root, "public", "stale.txt.zstd"), "stale")

	if _, err := buildDerivedAssets(root, assetsConfig{}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	encoded, err := os.ReadFile(filepath.Join(output, "app.css.zstd"))
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
	if _, err := os.Stat(filepath.Join(output, "image.png.zstd")); !os.IsNotExist(err) {
		t.Fatalf("binary sidecar exists: %v", err)
	}
	// A sidecar in the authored tree belongs to the build this one replaces.
	if _, err := os.Stat(filepath.Join(output, "stale.txt.zstd")); !os.IsNotExist(err) {
		t.Fatalf("stale sidecar was copied: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(root, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"package fixture",
		"middlewares.RegisterPublicManifest",
		`{URL: "app.css"`,
		`ContentEncoding: "zstd"`,
		`{URL: "image.png"`,
	} {
		if !strings.Contains(string(manifest), fragment) {
			t.Errorf("manifest missing %q:\n%s", fragment, manifest)
		}
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(assetManifestJSON))); err != nil {
		t.Fatalf("json manifest: %v", err)
	}
}

// TestBuildDerivedAssetsRefusesAnOldEmbed covers the migration: the scaffolded
// file is never rewritten by pw, so a project still embedding its authored tree
// is told what to change rather than serving bytes the build did not produce.
func TestBuildDerivedAssetsRefusesAnOldEmbed(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "public.go"), "package fixture\n\n//go:embed all:public\n")
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := buildDerivedAssets(root, assetsConfig{})
	if err == nil || !strings.Contains(err.Error(), derivedPublicDir) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildDerivedAssetsMinifiesAndRewritesCSS(t *testing.T) {
	root := derivedFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "public", "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "public", "logo.png"), "png")
	writeTestFile(t, filepath.Join(root, "public", "css", "site.css"),
		"body {\n  background-image: url(\"../logo.png\");\n  color: red;\n}\n")
	// A conversion is recognized by its output, so staging one is how a test
	// says the image was converted.
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "logo.webp"), "webp")

	report, err := buildDerivedAssets(root, assetsConfig{CSSMinify: true})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	stylesheet, err := os.ReadFile(filepath.Join(output, "css", "site.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stylesheet), "../logo.webp") {
		t.Errorf("url() was not rewritten: %s", stylesheet)
	}
	if strings.Contains(string(stylesheet), "\n  ") {
		t.Errorf("stylesheet was not minified: %s", stylesheet)
	}
	if _, err := os.Stat(filepath.Join(output, "logo.png")); !os.IsNotExist(err) {
		t.Errorf("converted source still ships: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "logo.webp")); err != nil {
		t.Errorf("produced file is missing: %v", err)
	}
	if len(report.converted) != 1 {
		t.Errorf("report.converted = %v", report.converted)
	}
}

// TestBuildDerivedAssetsRetainsAReferencedSource covers the one case where
// dropping a converted source would break a page: a reference the build cannot
// rewrite, such as one written in Go.
func TestBuildDerivedAssetsRetainsAReferencedSource(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "logo.png"), "png")
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "logo.webp"), "webp")
	writeNestedTestFile(t, filepath.Join(root, "handlers", "meta.go"),
		"package handlers\n\nconst ogImage = \"/public/logo.png\"\n")

	report, err := buildDerivedAssets(root, assetsConfig{})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	if _, err := os.Stat(filepath.Join(output, "logo.png")); err != nil {
		t.Errorf("retained source is missing: %v", err)
	}
	if len(report.retained) != 1 || !strings.Contains(report.retained[0], "handlers/meta.go") {
		t.Errorf("report.retained = %v", report.retained)
	}
}

func TestScaffoldIncludesPublicEmbed(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "fixture", TinyGo: true})
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

// TestBuildDerivedAssetsAddsAnAVIFVariant covers the one case where a URL
// carries two media types: the variant is a sibling of the file it competes
// with, never a URL of its own, because no markup selects it.
//
// The encoder is stated rather than run, because whether avif beats webp is a
// property of the image, and what is under test is the placement.
func TestBuildDerivedAssetsAddsAnAVIFVariant(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "photo.jpg"), "jpeg-source-bytes")
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "photo.webp"), "webp-bytes")

	smaller := func(source string, lossless bool, quality int) ([]byte, error) {
		return []byte("av"), nil
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	report, err := buildDerivedAssetsWithEncoder(root,
		assetsConfig{Images: true, AVIF: true, ImageQuality: defaultImageQuality}, smaller)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "photo.webp.avif")); err != nil {
		t.Fatalf("avif variant is missing: %v", err)
	}
	if len(report.skipped) != 0 {
		t.Errorf("report.skipped = %v", report.skipped)
	}
	manifest, err := os.ReadFile(filepath.Join(root, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	// One URL, two media types, and the fallback last so a client that reads
	// neither header still gets something it can decode.
	for _, fragment := range []string{
		`{URL: "photo.webp"`,
		`MediaType: "image/avif"`,
		`MediaType: "image/webp"`,
	} {
		if !strings.Contains(string(manifest), fragment) {
			t.Errorf("manifest missing %q:\n%s", fragment, manifest)
		}
	}
	if strings.Contains(string(manifest), `{URL: "photo.webp.avif"`) {
		t.Error("the variant became a URL of its own")
	}
}

// TestBuildDerivedAssetsDeclinesALargerAVIF is the other branch: a variant only
// earns its place by beating the representation it would be chosen over, and a
// decline is reported rather than silent.
func TestBuildDerivedAssetsDeclinesALargerAVIF(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "photo.jpg"), "jpeg-source-bytes")
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "photo.webp"), "webp")

	larger := func(source string, lossless bool, quality int) ([]byte, error) {
		return []byte("much-larger-than-the-webp"), nil
	}
	report, err := buildDerivedAssetsWithEncoder(root,
		assetsConfig{Images: true, AVIF: true, ImageQuality: defaultImageQuality}, larger)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	if _, err := os.Stat(filepath.Join(output, "photo.webp.avif")); !os.IsNotExist(err) {
		t.Errorf("a losing variant was written: %v", err)
	}
	if len(report.skipped) != 1 || !strings.Contains(report.skipped[0], "avif") {
		t.Errorf("report.skipped = %v", report.skipped)
	}
}

// TestGeneratedManifestIsValidGo keeps the emitter honest: it writes a file the
// application compiles, so a quoting mistake is a parse error here rather than
// in a project.
func TestGeneratedManifestIsValidGo(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), "body{}\n")
	if _, err := buildDerivedAssets(root, assetsConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, assetManifestFile), nil, parser.AllErrors); err != nil {
		t.Fatalf("generated manifest does not parse: %v", err)
	}
}
