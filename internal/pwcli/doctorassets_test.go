package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwcheck"
)

// assetCheckRun walks a public tree with the given switches and returns what
// the check reported.
func assetCheckRun(t *testing.T, assets assetsConfig, files map[string]string) []doctorFinding {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		writeNestedTestFile(t, filepath.Join(root, "public", filepath.FromSlash(name)), content)
	}
	state := projectState{}
	state.config.Assets = assets
	run := &checkRun{checkContext: checkContext{Env: "dev", Root: root, State: state}}
	run.checkAssetContent()
	return run.findings
}

func findingIDs(findings []doctorFinding) []string {
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.Check.ID)
	}
	return ids
}

// pw build fails on both conditions. Doctor is the form that reports without a
// build, and the only one that sees the tree server.public.read_local serves.
func TestDoctorReportsAssetContent(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			"the motivating case",
			map[string]string{"logo.png": scriptedSVG},
			[]string{pwcheck.AssetTypeMismatch},
		},
		{
			"an svg that executes",
			map[string]string{"icon.svg": scriptedSVG},
			[]string{pwcheck.AssetActiveSVG},
		},
		{
			"a signature-less extension holding a binary",
			map[string]string{"app.css": "PK\x03\x04archive"},
			[]string{pwcheck.AssetTypeMismatch},
		},
		{
			"an honest tree",
			map[string]string{"logo.png": pngHeader, "icon.svg": plainSVG, "app.css": "body{}"},
			nil,
		},
		{
			"an extension the table never heard of",
			map[string]string{"notes.rst": "PK\x03\x04archive"},
			nil,
		},
		{
			"both conditions in one tree",
			map[string]string{"logo.png": plainSVG, "icon.svg": scriptedSVG},
			[]string{pwcheck.AssetTypeMismatch, pwcheck.AssetActiveSVG},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := findingIDs(assetCheckRun(t, verifyingAssets(), testCase.files))
			if len(got) != len(testCase.want) {
				t.Fatalf("findings = %v, want %v", got, testCase.want)
			}
			for _, want := range testCase.want {
				if !strings.Contains(strings.Join(got, ","), want) {
					t.Errorf("findings = %v, want one to be %s", got, want)
				}
			}
		})
	}
}

// A sidecar in the authored tree is build output wherever it came from, and
// brotli has no signature at all, so judging one would only produce noise.
func TestDoctorSkipsSidecars(t *testing.T) {
	// The three suffixes the build actually writes. A .zst is not one of them
	// and is judged as an ordinary zstd file, which is why the name here is
	// .zstd rather than the shorter spelling.
	files := map[string]string{
		"app.css":      "body{}",
		"app.css.br":   "\x8b\x00\x80not brotli in any recognisable way",
		"app.css.gz":   "PK\x03\x04",
		"app.css.zstd": "PK\x03\x04",
	}
	if findings := assetCheckRun(t, verifyingAssets(), files); len(findings) != 0 {
		t.Errorf("findings = %v, want none", findingIDs(findings))
	}
}

func TestDoctorHonoursTheAllowList(t *testing.T) {
	assets := verifyingAssets()
	assets.VerifyAllow = []string{"vendor/**"}
	files := map[string]string{
		"vendor/widget.svg": scriptedSVG,
		"vendor/logo.png":   plainSVG,
	}
	if findings := assetCheckRun(t, assets, files); len(findings) != 0 {
		t.Errorf("findings = %v, want none", findingIDs(findings))
	}
}

// A project that switched both off is not walked at all.
func TestDoctorSkipsTheWalkWhenBothChecksAreOff(t *testing.T) {
	if findings := assetCheckRun(t, assetsConfig{}, map[string]string{"logo.png": scriptedSVG}); len(findings) != 0 {
		t.Errorf("findings = %v, want none", findingIDs(findings))
	}
}

// A tree that does not exist is not a finding: a project without a public
// directory has nothing to say here, and inventing one would be a different
// check's job.
func TestDoctorIsSilentWithoutAPublicTree(t *testing.T) {
	root := t.TempDir()
	run := &checkRun{checkContext: checkContext{Env: "dev", Root: root}}
	run.State.config.Assets = verifyingAssets()
	run.checkAssetContent()
	if len(run.findings) != 0 {
		t.Errorf("findings = %v, want none", findingIDs(run.findings))
	}
}

// Only the window is read for a file the SVG scan does not want, so a doctor
// run over a tree of large images does not read the tree.
func TestLeadingBytesReadsOnlyTheWindow(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "big.png")
	if err := os.WriteFile(name, []byte(pngHeader+strings.Repeat("payload", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	prefix, err := leadingBytes(name, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) != 64 {
		t.Errorf("read %d bytes, want the 64-byte window", len(prefix))
	}
	whole, err := leadingBytes(name, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(whole) <= len(prefix) {
		t.Errorf("the whole-file read returned %d bytes, want more than the window", len(whole))
	}
}

// A file shorter than the window is read to its end rather than padded, so a
// short file is not judged against trailing zeroes it does not have.
func TestLeadingBytesHandlesAShortFile(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "tiny.png")
	if err := os.WriteFile(name, []byte("ab"), 0o644); err != nil {
		t.Fatal(err)
	}
	prefix, err := leadingBytes(name, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(prefix) != "ab" {
		t.Errorf("prefix = %q, want %q", prefix, "ab")
	}
}
