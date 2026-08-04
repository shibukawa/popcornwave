package devconsole

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// textIsCompressible stands in for the build's own eligibility test. The pane
// takes the real one by injection, so this fixture only has to be a function of
// the same shape.
func textIsCompressible(path string) bool {
	switch filepath.Ext(path) {
	case ".css", ".js", ".html", ".json", ".svg":
		return true
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assetPage(t *testing.T, source AssetSource) string {
	t.Helper()
	if source.Compressible == nil {
		source.Compressible = textIsCompressible
	}
	recorder := httptest.NewRecorder()
	AssetPane(source).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "console template error") {
		t.Fatalf("the pane failed to render:\n%s", body)
	}
	return body
}

func TestAssetPaneReportsAnAbsentPublicDirectoryRatherThanFailing(t *testing.T) {
	body := assetPage(t, AssetSource{Root: t.TempDir(), Mount: "/public"})
	if !strings.Contains(body, "no <code>public</code> directory") {
		t.Errorf("an absent public tree was not reported as ordinary:\n%s", body)
	}
}

func TestAssetPaneNamesWhatAReleaseBuildWouldCompress(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", "app.css"), strings.Repeat("a{}", 200))
	writeFile(t, filepath.Join(root, "public", "logo.png"), "not really a png")

	body := assetPage(t, AssetSource{Root: root, Mount: "/public"})
	if !strings.Contains(body, "app.css") || !strings.Contains(body, "on build") {
		t.Errorf("an eligible file was not shown as one the build would compress:\n%s", body)
	}
	if !strings.Contains(body, "not compressible") {
		t.Errorf("an ineligible file was not named as such:\n%s", body)
	}
}

func TestAssetPaneResolvesTheURLThroughTheMount(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", "css", "app.css"), "a{}")

	body := assetPage(t, AssetSource{Root: root, Mount: "/static/"})
	if !strings.Contains(body, "/static/css/app.css") {
		t.Errorf("the pane never resolved the served URL:\n%s", body)
	}
}

// An undetermined value is said rather than replaced by a default, the way
// pw doctor reports what it could not read.
func TestAssetPaneSaysWhenTheMountIsUndetermined(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", "app.css"), "a{}")

	body := assetPage(t, AssetSource{Root: root})
	if !strings.Contains(body, "Not determined") || !strings.Contains(body, "mount is undetermined") {
		t.Errorf("an undetermined mount was not reported:\n%s", body)
	}
}

func TestAssetPaneReportsAStaleSidecar(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "public", "app.css")
	writeFile(t, source, "a{}")
	sidecar := source + ".zstd"
	writeFile(t, sidecar, "compressed")
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(sidecar, old, old); err != nil {
		t.Fatal(err)
	}

	body := assetPage(t, AssetSource{Root: root, Mount: "/public"})
	if !strings.Contains(body, "sidecar older than source") {
		t.Errorf("a stale sidecar went unreported:\n%s", body)
	}
}

// The developer loop never writes a sidecar, so one beside an ineligible file
// is left over from a build and would be removed by the next one.
func TestAssetPaneReportsASidecarABuildWouldRemove(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", "logo.png"), "binary")
	writeFile(t, filepath.Join(root, "public", "logo.png.zstd"), "compressed")

	body := assetPage(t, AssetSource{Root: root, Mount: "/public"})
	if !strings.Contains(body, "a build would remove it") {
		t.Errorf("an orphaned sidecar went unreported:\n%s", body)
	}
}

// The sentinel is embedded so that an empty tree still compiles, and it is
// never served, so it is not one of the project's assets.
func TestAssetPaneOmitsTheEmptyTreeSentinelAndSidecars(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", ".keep"), "")
	writeFile(t, filepath.Join(root, "public", "app.css"), "a{}")
	writeFile(t, filepath.Join(root, "public", "app.css.zstd"), "z")

	report := scanAssets(AssetSource{Root: root, Mount: "/public", Compressible: textIsCompressible})
	if len(report.Entries) != 1 || report.Entries[0].Path != "app.css" {
		t.Fatalf("entries = %+v, want only the source file", report.Entries)
	}
	if !report.Entries[0].SidecarPresent {
		t.Error("the sidecar was dropped instead of being folded into its source")
	}
}

func TestAssetPaneReportsStaleGeneratedCSS(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", "generated", "app.css"), "generated")
	writeFile(t, filepath.Join(root, "assets", "app.css"), "source")
	old := time.Now().Add(-time.Hour)
	output := filepath.Join(root, "public", "generated", "app.css")
	if err := os.Chtimes(output, old, old); err != nil {
		t.Fatal(err)
	}

	body := assetPage(t, AssetSource{
		Root: root, Mount: "/public",
		TailwindEnabled: true,
		TailwindInput:   "assets/app.css",
		TailwindOutput:  "public/generated/app.css",
	})
	if !strings.Contains(body, "older than") {
		t.Errorf("stale generated CSS went unreported:\n%s", body)
	}
}

// The pane reads the tree, so it answers whether or not the application is up.
// That is the whole reason it lives on the pw side of the boundary.
func TestAssetPaneNeedsNoRunningApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "public", "app.css"), "a{}")
	if body := assetPage(t, AssetSource{Root: root, Mount: "/public"}); !strings.Contains(body, "app.css") {
		t.Errorf("the pane did not answer from the tree alone:\n%s", body)
	}
}
