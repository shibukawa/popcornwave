package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinygodriver/fasthttp"
)

// A page whose markup belongs to one reader must not be held by a shared cache.
// The templates are what say so, and private is the default — so a render that
// declared nothing is private, and this half used to say nothing at all.
//
// It is the divergence a real application surfaced: two binaries of one project
// answered the same page, and only one of them told a proxy not to keep it.
func TestARenderedChainCarriesItsCachePolicy(t *testing.T) {
	for name, handler := range map[string]fasthttp.RequestHandler{
		"chain":    func(r *fasthttp.RequestCtx) { WriteHTMLChain(r, nil, staticFragment("<main>signed in</main>")) },
		"fragment": func(r *fasthttp.RequestCtx) { WriteHTMLFragment(r, staticFragment("<p>signed in</p>")) },
	} {
		_, header, _ := serve(t, handler, "/")
		if !strings.Contains(strings.ToLower(header), "cache-control: private, no-store") {
			t.Errorf("%s: a private render is cacheable by a shared cache:\n%s", name, header)
		}
	}
}
