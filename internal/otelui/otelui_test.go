package otelui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The UI is a committed build, so an empty or stale web/ directory would ship
// a viewer that serves nothing and fail only in front of a developer.
func TestHandlerServesTheCommittedBuild(t *testing.T) {
	handler := Handler()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, `<div id="root">`) {
		t.Fatalf("index.html is not the viewer page:\n%s", body)
	}

	// index.html references the hashed bundle by name, so following it proves
	// the assets were rebuilt together rather than left from an older build.
	script, ok := hashedAsset(body, `<script type="module" crossorigin src="`)
	if !ok {
		script, ok = hashedAsset(body, `<script type="module" src="`)
	}
	if !ok {
		t.Fatalf("index.html references no module bundle:\n%s", body)
	}
	assetResponse := httptest.NewRecorder()
	handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, script, nil))
	if assetResponse.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", script, assetResponse.Code, http.StatusOK)
	}
	bundle, err := io.ReadAll(assetResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle) == 0 {
		t.Fatalf("%s is empty", script)
	}
}

func hashedAsset(document, prefix string) (string, bool) {
	_, rest, found := strings.Cut(document, prefix)
	if !found {
		return "", false
	}
	path, _, found := strings.Cut(rest, `"`)
	if !found {
		return "", false
	}
	return "/" + strings.TrimPrefix(strings.TrimPrefix(path, "."), "/"), true
}
