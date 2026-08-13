package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

const svgBody = `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`

// serveVerifyTree answers one request against a manifest-less tree, which is
// the path a development loop and a local override both take.
func serveVerifyTree(t *testing.T, tree fstest.MapFS, config PublicAssetConfig, target string) *httptest.ResponseRecorder {
	t.Helper()
	config.Enabled = true
	if config.Mount == "" {
		config.Mount = "/public"
	}
	middleware, err := PublicAssets(config, tree)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	middleware(http.NotFoundHandler()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

// An SVG is the one served image that executes, and it executes in the
// application origin. The header does not depend on the build having read the
// file, which is what makes it the boundary rather than the scan.
func TestSVGResponseCarriesSandbox(t *testing.T) {
	if publicDevelopment {
		t.Skip("development mode intentionally ignores embedded assets")
	}
	tree := fstest.MapFS{
		"icon.svg": {Data: []byte(svgBody)},
		"app.css":  {Data: []byte("body{}")},
	}
	for _, testCase := range []struct {
		name, target string
		sandbox      bool
		want         string
	}{
		{"an svg is sandboxed", "/public/icon.svg", true, "sandbox"},
		{"the switch turns it off", "/public/icon.svg", false, ""},
		{"nothing else is sandboxed", "/public/app.css", true, ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := serveVerifyTree(t, tree, PublicAssetConfig{SVGSandbox: testCase.sandbox}, testCase.target)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := response.Header().Get("Content-Security-Policy"); got != testCase.want {
				t.Errorf("Content-Security-Policy = %q, want %q", got, testCase.want)
			}
		})
	}
}

// The manifest path answers from what the build decided, and the header is
// chosen from the media type it carries rather than from the URL, so an entry
// whose URL says .webp and whose bytes are SVG would still be sandboxed.
func TestSVGSandboxOnTheManifestPath(t *testing.T) {
	if publicDevelopment {
		t.Skip("the manifest is not consulted in development")
	}
	t.Cleanup(func() {
		publicManifestState.Lock()
		publicManifestState.entries = nil
		publicManifestState.Unlock()
	})
	RegisterPublicManifest([]AssetEntry{
		{URL: "icon.svg", CacheControl: "public, no-cache", Representations: []AssetRepresentation{
			{Path: "icon.svg", MediaType: "image/svg+xml", Length: len(svgBody), ETag: `"svg"`},
		}},
		{URL: "app.css", CacheControl: "public, no-cache", Representations: []AssetRepresentation{
			{Path: "app.css", MediaType: "text/css; charset=utf-8", Length: 6, ETag: `"css"`},
		}},
	})
	tree := fstest.MapFS{
		"icon.svg": {Data: []byte(svgBody)},
		"app.css":  {Data: []byte("body{}")},
	}
	if got := serveVerifyTree(t, tree, PublicAssetConfig{SVGSandbox: true}, "/public/icon.svg").
		Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("svg Content-Security-Policy = %q, want %q", got, "sandbox")
	}
	if got := serveVerifyTree(t, tree, PublicAssetConfig{SVGSandbox: true}, "/public/app.css").
		Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("css Content-Security-Policy = %q, want none", got)
	}
}

// A media type with parameters is still that media type.
func TestSVGSandboxIgnoresMediaTypeParameters(t *testing.T) {
	header := http.Header{}
	addSVGSandbox(header, "image/svg+xml; charset=utf-8", true)
	if got := header.Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want %q", got, "sandbox")
	}
}

// Add rather than Set. The security headers middleware writes the application's
// own policy, and replacing the field would drop it from every asset response;
// two policies are both enforced, so the sandbox can only tighten.
func TestSVGSandboxKeepsAnApplicationPolicy(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Security-Policy", "default-src 'self'")
	addSVGSandbox(header, "image/svg+xml", true)
	values := header.Values("Content-Security-Policy")
	if len(values) != 2 || values[0] != "default-src 'self'" || values[1] != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want the application policy then the sandbox", values)
	}
}

// The manifest-less path is the one serving bytes no build declared, so it is
// the one that has to look. A file whose bytes contradict its name is refused
// rather than answered with a Content-Type the bytes never earned.
func TestManifestLessPathRefusesMismatchedContent(t *testing.T) {
	if publicDevelopment {
		t.Skip("development mode intentionally ignores embedded assets")
	}
	tree := fstest.MapFS{
		// The motivating case: script-bearing SVG under a .png name.
		"logo.png": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>x</script></svg>`)},
		"real.svg": {Data: []byte(svgBody)},
		"app.css":  {Data: []byte("body{}")},
	}
	for _, testCase := range []struct {
		name, target string
		status       int
	}{
		{"a png holding svg is refused", "/public/logo.png", http.StatusInternalServerError},
		{"an honest svg still serves", "/public/real.svg", http.StatusOK},
		{"an ordinary stylesheet still serves", "/public/app.css", http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := serveVerifyTree(t, tree, PublicAssetConfig{SVGSandbox: true}, testCase.target)
			if response.Code != testCase.status {
				t.Errorf("status = %d, want %d", response.Code, testCase.status)
			}
		})
	}
}
