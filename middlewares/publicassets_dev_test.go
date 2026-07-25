//go:build pwdev

package middlewares

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestDevelopmentPublicAssetsUseOnlyLocalIdentity(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Mkdir("public", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("public", "app.css"), []byte("live"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("public", "app.css.zstd"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	middleware, err := PublicAssets(PublicAssetConfig{Mount: "/public"},
		fstest.MapFS{"embedded.txt": {Data: []byte("embedded")}})
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.NotFoundHandler())

	request := httptest.NewRequest(http.MethodGet, "/public/app.css", nil)
	request.Header.Set("Accept-Encoding", "zstd")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "live" ||
		response.Header().Get("Content-Encoding") != "" || response.Header().Get("Vary") != "" {
		t.Fatalf("response = %d %q %#v", response.Code, response.Body.String(), response.Header())
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/public/embedded.txt", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("embedded fallback status = %d", missing.Code)
	}
}
