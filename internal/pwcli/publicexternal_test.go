package pwcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwcheck"
)

// writeExternal puts a file in the second authored tree.
func writeExternal(t *testing.T, root, name, content string) {
	t.Helper()
	writeNestedTestFile(t, filepath.Join(root, externalPublicDir, filepath.FromSlash(name)), content)
}

// readManifestJSON returns the tooling copy, which carries everything the
// generated table does.
func readManifestJSON(t *testing.T, root string) []manifestEntryJSON {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(assetManifestJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var entries []manifestEntryJSON
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

func manifestEntryFor(t *testing.T, entries []manifestEntryJSON, url string) manifestEntryJSON {
	t.Helper()
	for _, entry := range entries {
		if entry.URL == url {
			return entry
		}
	}
	t.Fatalf("no manifest entry for %s", url)
	return manifestEntryJSON{}
}

// Not copying is the point of the tree. A staged copy would be byte-identical
// to its source, and these are the files a project least wants copied.
func TestExternalAssetsAreNotCopied(t *testing.T) {
	root := derivedFixture(t)
	writeExternal(t, root, "clip.mp4", "\x00\x00\x00\x20ftypisom and then some payload")
	if _, err := buildDerivedAssets(root, verifyingAssets()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(derivedPublicDir), "clip.mp4")); err == nil {
		t.Error("the external file was copied into the served tree")
	}
	entry := manifestEntryFor(t, readManifestJSON(t, root), "clip.mp4")
	if len(entry.Representations) != 1 {
		t.Fatalf("representations = %d, want 1", len(entry.Representations))
	}
	representation := entry.Representations[0]
	if !representation.External {
		t.Error("the entry is not marked external")
	}
	if representation.MediaType != "video/mp4" {
		t.Errorf("media type = %q, want video/mp4", representation.MediaType)
	}
	// No digest and no length: the build never read the file whole and does not
	// want to claim a validator the deployment could outlive.
	if representation.ETag != "" || representation.Length != 0 {
		t.Errorf("etag = %q length = %d, want neither", representation.ETag, representation.Length)
	}
}

// Nothing is converted or precompressed out here, which is what makes it the
// second tree rather than a second copy of the first.
func TestExternalAssetsGetNoSidecars(t *testing.T) {
	root := derivedFixture(t)
	writeExternal(t, root, "notes.txt", compressibleCSS)
	if _, err := buildDerivedAssets(root, verifyingAssets()); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".br", ".zstd", ".gz"} {
		if _, err := os.Stat(filepath.Join(root, externalPublicDir, "notes.txt"+suffix)); err == nil {
			t.Errorf("a %s sidecar was written into the external tree", suffix)
		}
	}
	entry := manifestEntryFor(t, readManifestJSON(t, root), "notes.txt")
	if len(entry.Representations) != 1 {
		t.Errorf("representations = %d, want only the identity one", len(entry.Representations))
	}
}

// The external tree wins, so the collision is defined rather than ambiguous and
// a warning is the honest severity. It is still worth saying: the embedded file
// remains on disk and remains the one an author is likely to open.
func TestExternalAssetShadowsAndWarns(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), compressibleCSS)
	writeExternal(t, root, "app.css", "external stylesheet")
	report, err := buildDerivedAssets(root, verifyingAssets())
	if err != nil {
		t.Fatalf("a collision failed the build: %v", err)
	}
	shadowed := strings.Join(report.shadowed, ",")
	for _, want := range []string{externalPublicDir + "/app.css", "public/app.css"} {
		if !strings.Contains(shadowed, want) {
			t.Errorf("shadowed = %v, want it to name %s", report.shadowed, want)
		}
	}
	entry := manifestEntryFor(t, readManifestJSON(t, root), "app.css")
	// Every representation of the shadowed URL goes, sidecars included: half an
	// entry from each tree would let one coding answer for the other's bytes.
	if len(entry.Representations) != 1 || !entry.Representations[0].External {
		t.Errorf("representations = %+v, want only the external one", entry.Representations)
	}
}

// The content checks run out here too, since the extension still decides the
// media type the response asserts.
func TestExternalAssetsAreVerified(t *testing.T) {
	root := derivedFixture(t)
	writeExternal(t, root, "logo.png", scriptedSVG)
	_, err := buildDerivedAssets(root, verifyingAssets())
	if err == nil {
		t.Fatal("the build accepted a mislabelled external file")
	}
	if !strings.Contains(err.Error(), "logo.png") {
		t.Errorf("error = %v, want it to name the file", err)
	}
}

// A project without the tree is the ordinary case, not a failure.
func TestBuildWithoutAnExternalTree(t *testing.T) {
	root := derivedFixture(t)
	writeTestFile(t, filepath.Join(root, "public", "app.css"), compressibleCSS)
	if _, err := buildDerivedAssets(root, verifyingAssets()); err != nil {
		t.Fatal(err)
	}
	for _, entry := range readManifestJSON(t, root) {
		for _, representation := range entry.Representations {
			if representation.External {
				t.Errorf("%s is marked external with no external tree", entry.URL)
			}
		}
	}
}

// The advisory points at the second tree; it never moves anything. Placement
// decides where bytes live, so the threshold only decides whether to speak.
func TestDoctorAdvisesMovingLargeMedia(t *testing.T) {
	root := t.TempDir()
	large := make([]byte, embeddedSizeAdvice+1)
	writeNestedTestFile(t, filepath.Join(root, "public", "clip.mp4"), string(large))
	writeNestedTestFile(t, filepath.Join(root, "public", "small.mp4"), "tiny")
	writeNestedTestFile(t, filepath.Join(root, "public", "data.json"), string(large))

	state := projectState{}
	state.config.Assets = verifyingAssets()
	run := &checkRun{checkContext: checkContext{Env: "dev", Root: root, State: state}}
	run.checkAssetContent()

	var advised []string
	for _, finding := range run.findings {
		if finding.Check.ID == pwcheck.AssetEmbeddedLarge {
			advised = append(advised, finding.Evidence)
		}
	}
	if len(advised) != 1 || advised[0] != "public/clip.mp4" {
		// A small media file is page furniture, and a large .json compresses,
		// so the embedded tree is doing real work for it.
		t.Errorf("advised = %v, want only public/clip.mp4", advised)
	}
}
