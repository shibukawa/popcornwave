package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The bytes a signature check is about. The PNG header is written as a literal
// rather than produced by an encoder, because what is under test is the table
// and not an image library.
const (
	pngHeader   = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	scriptedSVG = `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>x</script></svg>`
	plainSVG    = `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`
)

// verifyingAssets is the default a project gets, stated here so a test that
// changes one switch is obviously changing it.
func verifyingAssets() assetsConfig {
	return assetsConfig{Verify: true, VerifySVG: true}
}

// The build is the last moment before the tree is embedded, so both conditions
// fail it rather than warning.
func TestBuildDerivedAssetsRefusesMismatchedContent(t *testing.T) {
	for _, testCase := range []struct {
		name, file, content, want string
	}{
		{
			"the motivating case",
			"logo.png", scriptedSVG,
			"the extension declares png, and the bytes carry no png signature",
		},
		{
			"a stylesheet holding an archive",
			"app.css", "PK\x03\x04rest of the archive",
			"the extension declares a format with no signature, and the bytes carry zip",
		},
		{
			"an svg that executes",
			"icon.svg", scriptedSVG,
			`SVG carries "<script" at byte`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := derivedFixture(t)
			writeTestFile(t, filepath.Join(root, "public", testCase.file), testCase.content)
			_, err := buildDerivedAssets(root, verifyingAssets())
			if err == nil {
				t.Fatal("the build accepted the file")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error = %v, want it to contain %q", err, testCase.want)
			}
			if !strings.Contains(err.Error(), testCase.file) {
				t.Errorf("error = %v, want it to name %s", err, testCase.file)
			}
			// A refusal leaves no half-built tree carrying the file it refused.
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(derivedPublicDir), testCase.file)); err == nil {
				t.Error("the refused file reached the served tree")
			}
		})
	}
}

// The ordinary tree is the case that must stay silent, or the check is one
// projects turn off rather than one they benefit from.
func TestBuildDerivedAssetsAcceptsHonestContent(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "logo.png"), pngHeader)
	writeTestFile(t, filepath.Join(root, "public", "icon.svg"), plainSVG)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), compressibleCSS)
	writeTestFile(t, filepath.Join(root, "public", "notes.rst"), "an extension the table never heard of")
	report, err := buildDerivedAssets(root, verifyingAssets())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.exempted) != 0 {
		t.Errorf("exempted = %v, want none", report.exempted)
	}
}

// A build that produced a file labelled it, so the check runs on what the
// author wrote and not on what the build wrote back. The precompressed
// sidecars are the case that would otherwise fail: brotli has no signature at
// all, and .gz, .zstd, and .br are not extensions the tree owes an explanation
// for.
func TestBuildDerivedAssetsDoesNotJudgeItsOwnOutput(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), compressibleCSS)
	if _, err := buildDerivedAssets(root, verifyingAssets()); err != nil {
		t.Fatal(err)
	}
	// The sidecars exist, so a second build walks an authored tree that now
	// holds them. It has to stay silent about every one.
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(derivedPublicDir), "app.css.br")); err != nil {
		t.Fatalf("the fixture produced no sidecar to re-walk: %v", err)
	}
	if _, err := buildDerivedAssets(root, verifyingAssets()); err != nil {
		t.Fatalf("a second build refused its own output: %v", err)
	}
}

func TestBuildDerivedAssetsHonoursTheAllowList(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "logo.png"), scriptedSVG)
	writeNestedTestFile(t, filepath.Join(root, "public", "vendor", "widget.svg"), scriptedSVG)

	assets := verifyingAssets()
	assets.VerifyAllow = []string{"logo.png", "vendor/**"}
	report, err := buildDerivedAssets(root, assets)
	if err != nil {
		t.Fatalf("an exempted file was still refused: %v", err)
	}
	// An exemption is reported, because a list added for one bad file and
	// never removed is how a check quietly stops being one.
	exempted := strings.Join(report.exempted, ",")
	for _, want := range []string{"logo.png", "vendor/widget.svg"} {
		if !strings.Contains(exempted, want) {
			t.Errorf("exempted = %v, want it to name %s", report.exempted, want)
		}
	}
}

// Each switch turns off its own check and leaves the other running, so a
// project silencing one does not silence both.
func TestBuildDerivedAssetsSwitchesAreIndependent(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		assets  assetsConfig
		file    string
		content string
		refused bool
	}{
		{"the signature check off", assetsConfig{VerifySVG: true}, "logo.png", scriptedSVG, false},
		{"the svg scan still on", assetsConfig{VerifySVG: true}, "icon.svg", scriptedSVG, true},
		{"the svg scan off", assetsConfig{Verify: true}, "icon.svg", scriptedSVG, false},
		{"the signature check still on", assetsConfig{Verify: true}, "logo.png", scriptedSVG, true},
		{"both off", assetsConfig{}, "logo.png", scriptedSVG, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := derivedFixture(t)
			writeTestFile(t, filepath.Join(root, "public", testCase.file), testCase.content)
			_, err := buildDerivedAssets(root, testCase.assets)
			if refused := err != nil; refused != testCase.refused {
				t.Errorf("refused = %v (%v), want %v", refused, err, testCase.refused)
			}
		})
	}
}
