package middlewares

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// externalTree writes the second authored tree and moves into a working
// directory it resolves from, since the root is relative in the same way
// localPublicRoot is.
func externalTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	for name, content := range files {
		target := filepath.Join(root, filepath.FromSlash(externalPublicRoot), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func externalRequest(t *testing.T, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	middleware, err := PublicAssets(PublicAssetConfig{Enabled: true, Mount: "/public", SVGSandbox: true}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, target, nil)
	for key, value := range header {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	middleware(http.NotFoundHandler()).ServeHTTP(recorder, request)
	return recorder
}

// The whole reason the external location is a separate path: the kind of asset
// that belongs there is the kind a browser asks for in pieces, and a video that
// cannot seek is one that downloads the file to play from the middle.
func TestExternalAssetSupportsRange(t *testing.T) {
	body := strings.Repeat("0123456789", 32)
	externalTree(t, map[string]string{"clip.mp4": body})

	whole := externalRequest(t, "/public/clip.mp4", nil)
	if whole.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", whole.Code)
	}
	if got := whole.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes", got)
	}
	if whole.Body.String() != body {
		t.Errorf("body length = %d, want %d", whole.Body.Len(), len(body))
	}
	if got := whole.Header().Get("Last-Modified"); got == "" {
		t.Error("no Last-Modified, so the file has no validator of its own")
	}
	// No build-time validator: the tree is deployed as its own artifact, so an
	// ETag taken at build time could outlive the bytes it describes.
	if got := whole.Header().Get("ETag"); got != "" {
		t.Errorf("ETag = %q, want none for an external asset", got)
	}

	partial := externalRequest(t, "/public/clip.mp4", map[string]string{"Range": "bytes=10-19"})
	if partial.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", partial.Code)
	}
	if got := partial.Body.String(); got != body[10:20] {
		t.Errorf("body = %q, want %q", got, body[10:20])
	}
	if got := partial.Header().Get("Content-Range"); got != "bytes 10-19/320" {
		t.Errorf("Content-Range = %q, want bytes 10-19/320", got)
	}
}

func TestExternalAssetMediaTypeAndSandbox(t *testing.T) {
	externalTree(t, map[string]string{
		"clip.mp4": "\x00\x00\x00\x20ftypisom",
		"icon.svg": `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`,
	})
	video := externalRequest(t, "/public/clip.mp4", nil)
	if got := video.Header().Get("Content-Type"); got != "video/mp4" {
		t.Errorf("Content-Type = %q, want video/mp4", got)
	}
	if got := video.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("Content-Security-Policy = %q, want none", got)
	}
	// The sandbox follows the media type, not the tree, so an SVG that lives
	// out here is neutralised exactly as one that lives in the binary is.
	svg := externalRequest(t, "/public/icon.svg", nil)
	if got := svg.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want sandbox", got)
	}
}

// The external tree wins the URL in production, so it has to win in the loop.
// An author shadowing a file must not see one answer here and the other after
// a deploy, which is the layering mistake the local overlay already had to
// write a rule about.
func TestExternalAssetShadowsTheBuiltTree(t *testing.T) {
	root := externalTree(t, map[string]string{"app.css": "external"})
	built := filepath.Join(root, filepath.FromSlash(localPublicRoot))
	if err := os.MkdirAll(built, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(built, "app.css"), []byte("embedded"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := externalRequest(t, "/public/app.css", nil).Body.String(); got != "external" {
		t.Errorf("body = %q, want the external tree to win", got)
	}
}

// Path security is the same walk, so the refusals that hold for one tree hold
// for the other.
func TestExternalAssetRefusesASymlink(t *testing.T) {
	root := externalTree(t, nil)
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, filepath.FromSlash(externalPublicRoot), "leak.txt")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret.txt"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, found := externalAssetPath("leak.txt"); found {
		t.Error("a symbolic link resolved in the external tree")
	}
}

// A manifest naming a file the deployment did not carry means the binary and
// the tree beside it came from different places, and no response would be
// honest.
func TestManifestExternalEntryWithoutItsFileIs500(t *testing.T) {
	if publicDevelopment {
		t.Skip("the manifest is not consulted in development")
	}
	externalTree(t, nil)
	t.Cleanup(func() {
		publicManifestState.Store(nil)
	})
	RegisterPublicManifest([]AssetEntry{
		{URL: "gone.mp4", CacheControl: "public, no-cache", Representations: []AssetRepresentation{
			{Path: "gone.mp4", MediaType: "video/mp4", External: true},
		}},
	})
	if got := externalRequest(t, "/public/gone.mp4", nil).Code; got != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", got)
	}
}

// The manifest path serves the file and keeps the cache policy the build set,
// while the validator still comes from the file itself.
func TestManifestExternalEntryServesFromDisk(t *testing.T) {
	if publicDevelopment {
		t.Skip("the manifest is not consulted in development")
	}
	externalTree(t, map[string]string{"clip.mp4": "0123456789"})
	t.Cleanup(func() {
		publicManifestState.Store(nil)
	})
	RegisterPublicManifest([]AssetEntry{
		{URL: "clip.mp4", CacheControl: "public, no-cache", Representations: []AssetRepresentation{
			{Path: "clip.mp4", MediaType: "video/mp4", External: true},
		}},
	})
	response := externalRequest(t, "/public/clip.mp4", map[string]string{"Range": "bytes=2-4"})
	if response.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", response.Code)
	}
	if got := response.Body.String(); got != "234" {
		t.Errorf("body = %q, want %q", got, "234")
	}
	if got := response.Header().Get("Content-Type"); got != "video/mp4" {
		t.Errorf("Content-Type = %q, want the manifest's media type", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want the build's policy", got)
	}
}
