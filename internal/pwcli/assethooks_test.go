package pwcli

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// hashedURL matches a name carrying the digest segment that makes it immutable.
var hashedURL = regexp.MustCompile(`\.[0-9a-f]{12}\.[a-z]+(\.map)?$`)

// writeTestJPEG writes a lossy source, which is the axis the avif variant is
// worth measuring on: both formats are then re-encoding an approximation.
func writeTestJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8((x * y) % 256), G: uint8(y % 256), B: uint8(x % 256), A: 0xff})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, canvas, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	writeNestedTestFile(t, path, encoded.String())
}

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x % 256), G: 0x40, B: 0x80, A: 0xff})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		t.Fatal(err)
	}
	writeNestedTestFile(t, path, encoded.String())
}

// requireImageTools skips a test on a machine without the pinned encoders. The
// conversion is deliberately not implemented in Go: see imagetools.go.
func requireImageTools(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := resolveImageTool(name); err != nil {
			t.Skipf("%s is not installed", name)
		}
	}
}

func TestImageReferenceHookConverts(t *testing.T) {
	requireImageTools(t, webpTool)
	root := t.TempDir()
	writeTestPNG(t, filepath.Join(root, "public", "img", "logo.png"), 64, 64)

	hook := imageReferenceHook(root, assetsConfig{Images: true, ImageQuality: defaultImageQuality})
	if !hook.Match("/public/img/logo.png") || hook.Match("/public/img/logo.svg") {
		t.Fatal("hook claims the wrong references")
	}
	inputs, err := hook.CacheKey(htmlbind.ReferenceRequest{Value: "/public/img/logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	// The tool identity is what a cache hit would otherwise ignore after an
	// upgrade, so it has to be named here.
	if !strings.Contains(inputs.Params, webpTool) || len(inputs.Sources) != 1 {
		t.Fatalf("inputs = %+v", inputs)
	}
	result, err := hook.Transform(htmlbind.ReferenceRequest{Value: "/public/img/logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skip {
		t.Fatalf("conversion declined: %s", result.Reason)
	}
	// The name carries the digest of the bytes, which is what lets the response
	// promise immutability and be believed.
	if !hashedURL.MatchString(result.Value) || !strings.HasPrefix(result.Value, "/public/img/logo.") {
		t.Errorf("value = %q", result.Value)
	}
	if len(result.Files) != 1 || !hashedURL.MatchString(result.Files[0].Name) {
		t.Fatalf("files = %+v", result.Files)
	}
	if result.Files[0].MediaType != "image/webp" {
		t.Errorf("media type = %q", result.Files[0].MediaType)
	}
	if len(result.Head) != 0 {
		t.Errorf("an image contributed head entries: %+v", result.Head)
	}
}

// TestImageReferenceHookDeclinesALosingEncode covers the rule that a derived
// file larger than its source is not worth serving. The reference must stay on
// the authored file, and the reason must say why.
func TestImageReferenceHookDeclinesALosingEncode(t *testing.T) {
	root := t.TempDir()
	writeTestPNG(t, filepath.Join(root, "public", "dot.png"), 8, 8)

	// The comparison is what decides, so the test states an encoder that lost
	// rather than hunting for an image a real encoder happens to lose on.
	oversized := func(source string, lossless bool, quality int) ([]byte, error) {
		original, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		return make([]byte, len(original)+1), nil
	}
	result, err := convertImageReference(root, "/public/dot.png",
		assetsConfig{Images: true, ImageQuality: defaultImageQuality}, oversized)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skip || result.Reason == "" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Files) != 0 {
		t.Errorf("a declined conversion produced files: %+v", result.Files)
	}
}

func TestImageReferenceHookRefusesAMissingFile(t *testing.T) {
	root := t.TempDir()
	if _, err := imageReferenceHook(root, assetsConfig{Images: true, ImageQuality: defaultImageQuality}).Transform(htmlbind.ReferenceRequest{Value: "/public/absent.png"}); err == nil {
		t.Fatal("a reference resolving to no file was accepted")
	}
}

// TestScriptReferenceHookContributesHead is the driving case for the upstream
// head contribution: a TypeScript entry importing a stylesheet produces a file
// that no rewritten attribute can name, so the conversion has to declare its
// link itself.
func TestScriptReferenceHookContributesHead(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "public", "js", "app.ts"),
		"import \"./site.css\";\nconst greet = (name: string): string => `hi ${name}`;\nconsole.log(greet(\"world\"));\n")
	writeNestedTestFile(t, filepath.Join(root, "public", "js", "site.css"), "body { color: red }\n")

	result, err := buildScriptEntry(root, "/public/js/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !hashedURL.MatchString(result.Value) || !strings.HasPrefix(result.Value, "/public/js/app.") {
		t.Errorf("value = %q", result.Value)
	}
	kinds := map[string]string{}
	for _, file := range result.Files {
		kinds[path.Ext(file.Name)] = file.Name
	}
	for _, extension := range []string{".js", ".css", ".map"} {
		if kinds[extension] == "" {
			t.Fatalf("no %s output: %v", extension, kinds)
		}
	}
	if len(result.Head) != 1 {
		t.Fatalf("head = %+v", result.Head)
	}
	entry := result.Head[0]
	if entry.Element != "link" || entry.Attributes["rel"] != "stylesheet" ||
		!strings.HasSuffix(entry.Attributes["href"], path.Base(kinds[".css"])) {
		t.Errorf("head entry = %+v", entry)
	}
	// The stylesheet is named by no template, so only a reported read set makes
	// editing it regenerate.
	found := false
	for _, read := range result.Read {
		if strings.HasSuffix(read, filepath.Join("public", "js", "site.css")) {
			found = true
		}
	}
	if !found {
		t.Errorf("read set omits the imported stylesheet: %v", result.Read)
	}
}

func TestAssetTreePathStripsTheMount(t *testing.T) {
	for reference, expected := range map[string]string{
		"/public/img/a.png":  "img/a.png",
		"/assets/img/a.png":  "img/a.png",
		"/public/a.png?v=2":  "a.png",
		"img/a.png":          "img/a.png",
		"/public":            "",
		"/public/a.png#frag": "a.png",
	} {
		if got := assetTreePath(reference); got != expected {
			t.Errorf("assetTreePath(%q) = %q, want %q", reference, got, expected)
		}
	}
}

// TestAuthoredAssetPathStaysInTheTree covers the containment rule: a reference
// that climbs out of public resolves to nothing rather than to a file beside it.
func TestAuthoredAssetPathStaysInTheTree(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "secret.png"), "png")
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := authoredAssetPath(root, "/public/../secret.png"); err == nil {
		t.Fatal("a traversing reference resolved")
	}
}

// TestWebPHasNoPlatformFallback records why the two formats differ: sips writes
// avif and only reads webp, so listing it for webp would produce a command that
// exits zero and writes nothing.
func TestWebPHasNoPlatformFallback(t *testing.T) {
	for _, lossless := range []bool{true, false} {
		if got := encoderCandidates("webp", lossless); len(got) != 1 || got[0] != webpTool {
			t.Errorf("webp candidates (lossless=%v) = %v", lossless, got)
		}
	}
	avif := encoderCandidates("avif", false)
	if avif[0] != avifTool {
		t.Errorf("the dedicated encoder is not preferred: %v", avif)
	}
	if runtime.GOOS == "darwin" {
		if len(avif) != 2 || avif[1] != sipsTool {
			t.Errorf("macOS lossy avif candidates = %v", avif)
		}
	}
}

// TestLosslessAxisExcludesThePlatformTool is the rule a png depends on.
//
// sips accepts "formatOptions lossless" and writes a lossy file anyway:
// measured on 2026-08-04, a png round tripped through it came back with pixels
// differing by up to 8 across most of the image, where avifenc --lossless came
// back byte-identical. A png is authored exact, so an encoder that cannot keep
// it exact must not be reachable from that axis at all.
func TestLosslessAxisExcludesThePlatformTool(t *testing.T) {
	for _, candidate := range encoderCandidates("avif", true) {
		if candidate == sipsTool {
			t.Fatalf("the platform tool is reachable from the lossless axis: %v",
				encoderCandidates("avif", true))
		}
	}
}

// TestLosslessAVIFRoundTripsExactly is the measurement itself, kept as a test
// so an encoder upgrade that quietly stops being lossless is caught here rather
// than in someone's screenshots.
func TestLosslessAVIFRoundTripsExactly(t *testing.T) {
	requireImageTools(t, avifTool)
	root := t.TempDir()
	source := filepath.Join(root, "diagram.png")
	writeTestPNG(t, source, 64, 64)
	encoded, err := encodeAVIF(source, true, defaultImageQuality)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	// Size is not what this test is about: whether a lossless variant beats its
	// source is a property of the image, and the size comparison in the build
	// is where that is decided.
	if len(encoded) == 0 {
		t.Fatal("the encoder produced nothing")
	}
	decoded, err := decodeToPNG(t, encoded)
	if err != nil {
		t.Skipf("no decoder available to verify the round trip: %v", err)
	}
	before, after := decodeImage(t, original), decodeImage(t, decoded)
	if !sameRGB(before, after) {
		t.Error("a lossless conversion changed pixels")
	}
}

// TestSipsWritesAVIF exercises the fallback itself on the platform that has it.
func TestSipsWritesAVIF(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the platform tool is macOS only")
	}
	identity, err := resolveImageTool(sipsTool)
	if err != nil {
		t.Skipf("%s is unavailable: %v", sipsTool, err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "photo.jpg")
	writeTestJPEG(t, source, 96, 96)
	encoded, err := runSips(identity, source, ".avif", "avif", defaultImageQuality)
	if err != nil {
		t.Fatal(err)
	}
	// An avif file starts with an ftyp box naming the brand.
	if len(encoded) < 12 || string(encoded[4:8]) != "ftyp" {
		t.Fatalf("output is not an ISO media file: %d bytes", len(encoded))
	}
}

// TestMissingEncoderDeclinesRatherThanFailing is what makes the diagnostic a
// warning: an unconverted image is a larger page, not a broken one.
func TestMissingEncoderDeclinesRatherThanFailing(t *testing.T) {
	root := t.TempDir()
	writeTestPNG(t, filepath.Join(root, "public", "logo.png"), 32, 32)
	absent := func(source string, lossless bool, quality int) ([]byte, error) {
		return nil, errNoEncoder
	}
	result, err := convertImageReference(root, "/public/logo.png",
		assetsConfig{Images: true, ImageQuality: defaultImageQuality}, absent)
	if err != nil {
		t.Fatalf("a missing encoder failed the build: %v", err)
	}
	if !result.Skip || !strings.Contains(result.Reason, "encoder") {
		t.Errorf("result = %+v", result)
	}
}

// decodeToPNG converts encoded image bytes back through the platform tool,
// which is the only decoder available here for these formats.
func decodeToPNG(t *testing.T, encoded []byte) ([]byte, error) {
	t.Helper()
	identity, err := resolveImageTool(sipsTool)
	if err != nil {
		return nil, err
	}
	source := filepath.Join(t.TempDir(), "input.avif")
	if err := os.WriteFile(source, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return runSipsFormat(identity, source, ".png", "png")
}

// runSipsFormat is the decode direction, which takes no quality setting.
func runSipsFormat(identity toolIdentity, source, extension, format string) ([]byte, error) {
	return runImageToolWith(identity, source, extension, []string{"-s", "format", format},
		func(target string) []string { return []string{"--out", target} }, sipsTool)
}

func decodeImage(t *testing.T, encoded []byte) image.Image {
	t.Helper()
	decoded, _, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return decoded
}

// sameRGB compares colour without alpha, because a decode through the platform
// tool may drop an alpha channel the comparison is not about.
func sameRGB(before, after image.Image) bool {
	bounds := before.Bounds()
	if !after.Bounds().Eq(bounds) {
		return false
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := before.At(x, y).RGBA()
			r2, g2, b2, _ := after.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				return false
			}
		}
	}
	return true
}

// TestScriptBuildEmitsAModule pins the output format against the tag rule
// verifyScriptModuleTags enforces. The two are one decision: the build emits a
// module, and the check is what guarantees a module tag is there to run it.
func TestScriptBuildEmitsAModule(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "public", "js", "app.ts"),
		"export const version = \"1\";\nconst el = document.querySelector(\"#app\");\nif (el) el.textContent = `v${version}`;\n")

	result, err := buildScriptEntry(root, "/public/js/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	var bundle string
	for _, file := range result.Files {
		if strings.HasSuffix(file.Name, ".js") {
			bundle = string(file.Content)
		}
	}
	if bundle == "" {
		t.Fatal("no bundle was produced")
	}
	if !strings.Contains(bundle, "export") {
		t.Errorf("the bundle is not a module: %q", bundle)
	}
}

// TestHooksLeaveAnotherOriginAlone is the rule that keeps a CDN reference from
// failing a build: claiming it means resolving it under public, and a URL that
// was always correct would then be a generation error.
func TestHooksLeaveAnotherOriginAlone(t *testing.T) {
	root := t.TempDir()
	image := imageReferenceHook(root, assetsConfig{Images: true, ImageQuality: defaultImageQuality})
	script := scriptReferenceHook(root)
	for _, value := range []string{
		"https://cdn.example.com/img/a.png",
		"//cdn.example.com/img/a.png",
		"data:image/png;base64,AAAA",
	} {
		if image.Match(value) {
			t.Errorf("the image hook claimed %q", value)
		}
	}
	for _, value := range []string{
		"https://cdn.example.com/js/app.ts",
		"//cdn.example.com/js/app.ts",
	} {
		if script.Match(value) {
			t.Errorf("the script hook claimed %q", value)
		}
	}
	// The local forms still match, or the exclusion would have removed the
	// feature rather than the hazard.
	if !image.Match("/public/img/a.png") || !script.Match("/public/js/app.ts") {
		t.Error("a local reference stopped matching")
	}
}

// TestScriptReadSetHoldsOnlyRealInputs guards the incremental skip.
//
// The metafile lists outputs beside inputs, and an earlier line-scanning
// version swept those in. A recorded dependency that is not a file on disk is
// unverifiable, and an unverifiable record regenerates, so every build would
// have converted everything again while appearing to cache.
func TestScriptReadSetHoldsOnlyRealInputs(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "public", "js", "app.ts"),
		"import { hello } from \"./util\";\nconsole.log(hello(\"x\"));\n")
	writeNestedTestFile(t, filepath.Join(root, "public", "js", "util.ts"),
		"export const hello = (n: string): string => `hi ${n}`;\n")

	result, err := buildScriptEntry(root, "/public/js/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Read) == 0 {
		t.Fatal("the read set is empty")
	}
	for _, name := range result.Read {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("read set names something that is not a file: %s", name)
		}
	}
	// Sorted, because the set joins a cache key and a map iterates in no order.
	if !sort.StringsAreSorted(result.Read) {
		t.Errorf("read set is unordered: %v", result.Read)
	}
}

// TestVariantCacheSkipsTheSecondEncode is the cost this cache exists for: a
// variant is converted by the tree walk, so the upstream conversion cache never
// sees it, and every build re-encoded every image whether or not anything had
// changed.
func TestVariantCacheSkipsTheSecondEncode(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "photo.jpg")
	writeTestJPEG(t, source, 48, 48)
	calls := 0
	counted := func(name string, lossless bool, quality int) ([]byte, error) {
		calls++
		return []byte("encoded-bytes"), nil
	}
	encode := cachedImageEncoder(filepath.Join(root, "cache"), "avif", counted)

	for range 3 {
		encoded, err := encode(source, false, defaultImageQuality)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != "encoded-bytes" {
			t.Fatalf("encoded = %q", encoded)
		}
	}
	if calls != 1 {
		t.Errorf("the encoder ran %d times", calls)
	}

	// Everything the output depends on has to miss: an edited source, and a
	// different setting on the same source.
	writeTestJPEG(t, source, 64, 64)
	if _, err := encode(source, false, defaultImageQuality); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("an edited source hit the cache: %d calls", calls)
	}
	if _, err := encode(source, false, defaultImageQuality-10); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("a changed quality hit the cache: %d calls", calls)
	}
}

// TestScriptBundleLinksItsSourceMap covers the one ordering problem hashing
// creates: the bundle names its map in a trailing comment, so the digest is
// taken over the bundle without it and the comment is written back naming the
// map that digest produced.
func TestScriptBundleLinksItsSourceMap(t *testing.T) {
	root := t.TempDir()
	writeNestedTestFile(t, filepath.Join(root, "public", "js", "app.ts"),
		"const greet = (name: string): string => `hi ${name}`;\nconsole.log(greet(\"world\"));\n")

	result, err := buildScriptEntry(root, "/public/js/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	var bundle, sourcemap htmlbind.ProducedFile
	for _, file := range result.Files {
		switch path.Ext(file.Name) {
		case ".js":
			bundle = file
		case ".map":
			sourcemap = file
		}
	}
	if bundle.Name == "" || sourcemap.Name == "" {
		t.Fatalf("files = %+v", result.Files)
	}
	if sourcemap.Name != bundle.Name+".map" {
		t.Errorf("the map does not follow the bundle: %q and %q", bundle.Name, sourcemap.Name)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(bundle.Content)),
		"//# sourceMappingURL="+path.Base(sourcemap.Name)) {
		t.Errorf("the bundle does not link its map: %q", bundle.Content)
	}
}

// TestHashedNameFollowsTheBytes is the property the immutable cache header
// rests on: different bytes are a different URL, so a response that promises
// never to change can be believed.
func TestHashedNameFollowsTheBytes(t *testing.T) {
	first := hashedName("img/logo.webp", []byte("one"))
	again := hashedName("img/logo.webp", []byte("one"))
	other := hashedName("img/logo.webp", []byte("two"))
	if first != again {
		t.Errorf("the same bytes produced two names: %q and %q", first, again)
	}
	if first == other {
		t.Errorf("different bytes produced one name: %q", first)
	}
	if !hashedURL.MatchString(first) || !strings.HasPrefix(first, "img/logo.") {
		t.Errorf("name = %q", first)
	}
	// The tree builder maps a produced file back to its source by removing the
	// segment, so the two have to agree on what one looks like.
	if contentHashOf(strings.TrimSuffix(first, ".webp")) == "" {
		t.Errorf("the digest segment is not recognized in %q", first)
	}
}
