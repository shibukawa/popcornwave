//go:build !pwdev

package pwfast

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/shibukawa/popcornweb/middlewares"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// TestBothTransportsAgreeOnTheRevisionedPolicy is why the resolution lives in
// the shared leaf rather than in each transport.
//
// The two are separate implementations of reading a request and writing a
// response, and the cache policy is the one decision neither may make for
// itself: this half serving the revalidating policy for a URL a document
// promised was immutable would show up as nothing at all — the page would
// render, and every load would spend a round trip.
func TestBothTransportsAgreeOnTheRevisionedPolicy(t *testing.T) {
	// The registry is process-wide, so the manifest comes back off before
	// anything that runs after this asks the tree for a name it does not hold.
	t.Cleanup(func() { middlewares.RegisterPublicManifest(nil) })
	middlewares.RegisterPublicManifest([]middlewares.AssetEntry{
		{URL: "site.css", CacheControl: "public, no-cache", Revision: "0123456789abcdef",
			Representations: []middlewares.AssetRepresentation{
				{Path: "site.css", MediaType: "text/css; charset=utf-8", Length: 6, ETag: `"css"`},
			}},
	})
	handler := assetHandler(t, fstest.MapFS{"site.css": {Data: []byte("body{}")}})

	for _, testCase := range []struct{ target, cacheControl string }{
		{"/static/site.css", "public, no-cache"},
		{"/static/0123456789abcdef/site.css", middlewares.RevisionedCacheControl},
	} {
		status, header, body := serve(t, handler, testCase.target)
		if status != fasthttp.StatusOK || body != "body{}" {
			t.Fatalf("%s answered %d %q", testCase.target, status, body)
		}
		if !strings.Contains(header, "Cache-Control: "+testCase.cacheControl+"\r\n") {
			t.Errorf("%s did not carry %q:\n%s", testCase.target, testCase.cacheControl, header)
		}
	}
	// A segment this build does not serve is not answered from the current tree,
	// which is what a browser holding the old URL forever depends on.
	if status, _, _ := serve(t, handler, "/static/ffffffffffffffff/site.css"); status != fasthttp.StatusNotFound {
		t.Errorf("a stale revision answered %d", status)
	}
}
