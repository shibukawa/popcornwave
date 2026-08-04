package middlewares

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPublicAssetHandlerEmbeddedAndNegotiation(t *testing.T) {
	if publicDevelopment {
		t.Skip("development mode intentionally ignores embedded assets")
	}
	embedded := fstest.MapFS{
		"app.css":         {Data: []byte("body{}")},
		"app.css.zstd":    {Data: []byte("encoded")},
		"docs/index.html": {Data: []byte("<h1>docs</h1>")},
	}
	middleware, err := PublicAssets(PublicAssetConfig{Enabled: true, Mount: "/public"}, embedded)
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.NotFoundHandler())

	tests := []struct {
		name, method, target, encoding string
		status                         int
		body, contentEncoding          string
	}{
		{name: "identity", method: http.MethodGet, target: "/public/app.css", status: 200, body: "body{}"},
		{name: "zstd", method: http.MethodGet, target: "/public/app.css", encoding: "gzip, zstd", status: 200, body: "encoded", contentEncoding: "zstd"},
		{name: "wildcard", method: http.MethodGet, target: "/public/app.css", encoding: "*;q=0.5", status: 200, body: "encoded", contentEncoding: "zstd"},
		{name: "zstd disabled", method: http.MethodGet, target: "/public/app.css", encoding: "zstd;q=0", status: 200, body: "body{}"},
		{name: "not acceptable", method: http.MethodGet, target: "/public/app.css", encoding: "identity;q=0, zstd;q=0", status: 406},
		{name: "head", method: http.MethodHead, target: "/public/app.css", encoding: "zstd", status: 200, contentEncoding: "zstd"},
		{name: "index", method: http.MethodGet, target: "/public/docs", status: 200, body: "<h1>docs</h1>"},
		{name: "sidecar hidden", method: http.MethodGet, target: "/public/app.css.zstd", status: 404},
		{name: "dot hidden", method: http.MethodGet, target: "/public/.keep", status: 404},
		{name: "method", method: http.MethodPost, target: "/public/app.css", status: 405},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, nil)
			if test.encoding != "" {
				request.Header.Set("Accept-Encoding", test.encoding)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || test.body != "" && response.Body.String() != test.body {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Content-Encoding"); got != test.contentEncoding {
				t.Fatalf("Content-Encoding = %q", got)
			}
			if (test.status == http.StatusOK || test.status == http.StatusNotAcceptable) &&
				response.Header().Get("Vary") != "Accept-Encoding" {
				t.Fatalf("Vary = %q", response.Header().Get("Vary"))
			}
			if test.method == http.MethodHead && response.Body.Len() != 0 {
				t.Fatalf("HEAD body = %q", response.Body.String())
			}
		})
	}
}

func TestPublicAssetHandlerRedirectAndRejectsUnsafePaths(t *testing.T) {
	middleware, err := PublicAssets(PublicAssetConfig{Enabled: true, Mount: "/assets/"}, fstest.MapFS{
		"ok.txt": {Data: []byte("ok")},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.NotFoundHandler())
	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/assets?version=1", nil))
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/assets/?version=1" {
		t.Fatalf("redirect = %d %q", redirect.Code, redirect.Header().Get("Location"))
	}
	for _, target := range []string{"/assets/../ok.txt", "/assets/a/../../ok.txt", "/assets/%2e%2e/ok.txt", "/assets/a%5cb.txt"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d", target, response.Code)
		}
	}
}

func TestReadLocalPublicAssetRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.MkdirAll(filepath.FromSlash(localPublicRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret.txt"), filepath.Join(root, filepath.FromSlash(localPublicRoot), "leak.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, found, rejected := readLocalPublicAsset("leak.txt"); found || !rejected {
		t.Fatalf("found=%v rejected=%v", found, rejected)
	}
}

func TestPublicAssetLocalOverlayIsLayerConsistent(t *testing.T) {
	if publicDevelopment {
		t.Skip("production overlay behavior")
	}
	root := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.MkdirAll(filepath.FromSlash(localPublicRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.FromSlash(localPublicRoot), "app.css"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	embedded := fstest.MapFS{
		"app.css":      {Data: []byte("embedded")},
		"app.css.zstd": {Data: []byte("embedded-zstd")},
		"fallback.txt": {Data: []byte("fallback")},
	}
	asset, ok := resolvePublicAsset("app.css", PublicAssetConfig{ReadLocal: true}, embedded)
	if !ok || string(asset.identity) != "local" || asset.zstd != nil {
		t.Fatalf("local asset = %#v, %v", asset, ok)
	}
	asset, ok = resolvePublicAsset("fallback.txt", PublicAssetConfig{ReadLocal: true}, embedded)
	if !ok || string(asset.identity) != "fallback" {
		t.Fatalf("fallback asset = %#v, %v", asset, ok)
	}
}

func TestNormalizePublicMount(t *testing.T) {
	for _, input := range []string{"/public", "/public/"} {
		if got, err := NormalizePublicMount(input); err != nil || got != "/public/" {
			t.Fatalf("%q => %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "/", "public", "/a/../b", "/a//b", "/a*", "/a?b"} {
		if _, err := NormalizePublicMount(input); err == nil {
			t.Errorf("%q unexpectedly valid", input)
		}
	}
}

func TestEmbeddedAssetRequiresRegularFile(t *testing.T) {
	var embedded fs.FS = fstest.MapFS{"directory/file.txt": {Data: []byte("ok")}}
	if _, ok := readEmbeddedPublicAsset(embedded, "directory"); ok {
		t.Fatal("directory without index was served")
	}
	if _, ok := publicAssetName(strings.Repeat("a", 1)); !ok {
		t.Fatal("valid asset name rejected")
	}
}
