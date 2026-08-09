package pwcli

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
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

// compressibleCSS is long enough that every coding beats the source. A short
// document does not: frame overhead alone exceeds what there is to save, and
// TestBuildDerivedAssetsSkipsUnprofitableCodings covers that side.
const compressibleCSS = `body { color: red }
.a { color: red }
.b { color: red }
.c { color: red }
.d { color: red }
.e { color: red }
.f { color: red }
.g { color: red }
`

func TestBuildDerivedAssets(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), compressibleCSS)
	writeTestFile(t, filepath.Join(root, "public", "image.png"), "png")
	writeTestFile(t, filepath.Join(root, "public", "stale.txt.zstd"), "stale")

	if _, err := buildDerivedAssets(root, assetsConfig{}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	// One sidecar per coding, each decoding back to the same source.
	for _, sidecar := range []struct {
		suffix string
		decode func([]byte) ([]byte, error)
	}{
		{suffix: ".br", decode: decodeTestBrotli},
		{suffix: ".zstd", decode: decodeTestZstd},
		{suffix: ".gz", decode: decodeTestGzip},
	} {
		encoded, err := os.ReadFile(filepath.Join(output, "app.css"+sidecar.suffix))
		if err != nil {
			t.Fatalf("%s sidecar: %v", sidecar.suffix, err)
		}
		if len(encoded) >= len(compressibleCSS) {
			t.Errorf("%s sidecar is not smaller than its source: %d >= %d", sidecar.suffix, len(encoded), len(compressibleCSS))
		}
		decoded, err := sidecar.decode(encoded)
		if err != nil || string(decoded) != compressibleCSS {
			t.Errorf("%s decoded = %q, %v", sidecar.suffix, decoded, err)
		}
	}
	for _, suffix := range []string{".br", ".zstd", ".gz"} {
		if _, err := os.Stat(filepath.Join(output, "image.png"+suffix)); !os.IsNotExist(err) {
			t.Errorf("binary %s sidecar exists: %v", suffix, err)
		}
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
		`ContentEncoding: "br"`,
		`ContentEncoding: "zstd"`,
		`ContentEncoding: "gzip"`,
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

// TestBuildDerivedAssetsSkipsUnprofitableCodings covers the other half of the
// rule: an encode that comes out no smaller than its source is not written, so
// the embed carries no file that could only ever lose a negotiation.
func TestBuildDerivedAssetsSkipsUnprofitableCodings(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "tiny.txt"), "a\n")

	if _, err := buildDerivedAssets(root, assetsConfig{}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	if _, err := os.Stat(filepath.Join(output, "tiny.txt")); err != nil {
		t.Fatalf("source is missing: %v", err)
	}
	for _, suffix := range []string{".br", ".zstd", ".gz"} {
		if _, err := os.Stat(filepath.Join(output, "tiny.txt"+suffix)); !os.IsNotExist(err) {
			t.Errorf("%s sidecar was written for a file it cannot shrink: %v", suffix, err)
		}
	}
}

func decodeTestZstd(encoded []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return decoder.DecodeAll(encoded, nil)
}

func decodeTestGzip(encoded []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func decodeTestBrotli(encoded []byte) ([]byte, error) {
	return io.ReadAll(brotli.NewReader(bytes.NewReader(encoded)))
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

// TestBuildDerivedAssetsDropsAConvertedTSXEntry is the second half of building a
// tsx entry: a source the bundle replaced must leave the served tree, or the
// authored JSX ships beside the output it was compiled into.
func TestBuildDerivedAssetsDropsAConvertedTSXEntry(t *testing.T) {
	root := derivedFixture(t)
	writeNestedTestFile(t, filepath.Join(root, "public", "islands", "counter.tsx"),
		"export const label = <b>hi</b>;\n")
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "islands", "counter.js"),
		"export const label=1;\n")

	report, err := buildDerivedAssets(root, assetsConfig{Scripts: true})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	if _, err := os.Stat(filepath.Join(output, "islands", "counter.tsx")); !os.IsNotExist(err) {
		t.Errorf("the authored tsx entry still ships: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "islands", "counter.js")); err != nil {
		t.Errorf("the bundle is missing: %v", err)
	}
	if len(report.converted) != 1 || !strings.Contains(report.converted[0], "counter.tsx") {
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

// TestBuildDerivedAssetsShipsWhatIsStaged pins the contract the staging
// directory has with generation: everything found there reaches the served
// tree, which is why generation clears it before writing.
//
// Without that, a file produced for a source since deleted would keep being
// copied in, with a manifest entry and a URL, on every build after the source
// was gone.
func TestBuildDerivedAssetsShipsWhatIsStaged(t *testing.T) {
	root := derivedFixture(t)
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "js", "orphan.js"), "console.log(1)")

	if _, err := buildDerivedAssets(root, assetsConfig{}); err != nil {
		t.Fatal(err)
	}
	served := filepath.Join(root, filepath.FromSlash(derivedPublicDir), "js", "orphan.js")
	if _, err := os.Stat(served); err != nil {
		t.Fatalf("a staged file did not reach the tree: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(root, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `{URL: "js/orphan.js"`) {
		t.Errorf("manifest missing the staged file:\n%s", manifest)
	}
}

// TestManifestPromisesImmutabilityOnlyForInventedURLs is the rule the two cache
// policies rest on: a name carrying its own digest can promise never to change,
// and a name the author wrote serves whatever the next build puts behind it.
func TestManifestPromisesImmutabilityOnlyForInventedURLs(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), "body{}\n")
	writeNestedTestFile(t, filepath.Join(root, filepath.FromSlash(derivedStageDir), "js", "app.abcdef012345.js"), "console.log(1)")

	if _, err := buildDerivedAssets(root, assetsConfig{}); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(assetManifestJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		URL          string `json:"url"`
		CacheControl string `json:"cache_control"`
	}
	if err := json.Unmarshal(manifest, &entries); err != nil {
		t.Fatal(err)
	}
	policies := map[string]string{}
	for _, entry := range entries {
		policies[entry.URL] = entry.CacheControl
	}
	if got := policies["js/app.abcdef012345.js"]; got != immutableCacheControl {
		t.Errorf("a produced URL is not immutable: %q", got)
	}
	if got := policies["app.css"]; got != derivedCacheControl {
		t.Errorf("an authored URL claims immutability: %q", got)
	}
}
