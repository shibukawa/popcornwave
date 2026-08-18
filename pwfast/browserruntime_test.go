package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwbrowser"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

func assets() fasthttp.RequestHandler {
	return Compose(func(r *fasthttp.RequestCtx) { _, _ = r.WriteString("application") },
		Frame{Slot: SlotOperational, Middleware: FrameworkAssets()})
}

// A document names the runtime through the shared leaf, so the URL it writes is
// the one this transport has to answer. Without this the second build rendered
// a page whose script tag pointed at a 404: both server halves of the update
// surface worked and nothing client-side could reach them.
func TestTheBrowserRuntimeIsServedAtTheURLADocumentNames(t *testing.T) {
	status, header, body := serve(t, assets(), pwbrowser.RuntimeScriptURL())
	if status != fasthttp.StatusOK {
		t.Fatalf("status = %d for the URL a document names", status)
	}
	if body != pwbrowser.Scripts()[pwbrowser.RuntimeName] {
		t.Errorf("the bytes served are not the module in the set (%d vs %d)",
			len(body), len(pwbrowser.Scripts()[pwbrowser.RuntimeName]))
	}
	lower := strings.ToLower(header)
	if !strings.Contains(lower, "text/javascript") {
		t.Errorf("the module is not served as a script:\n%s", header)
	}
	// The revision segment never serves different bytes, so this is genuinely
	// immutable rather than merely long-lived.
	if !strings.Contains(lower, "immutable") {
		t.Errorf("the asset is not cacheable forever:\n%s", header)
	}
}

// A stale revision is not found rather than served from the current set, which
// is what makes the immutable caching sound.
func TestAStaleRevisionIsNotServedFromTheCurrentSet(t *testing.T) {
	status, _, _ := serve(t, assets(), pwbrowser.Prefix+"0000000000000000/"+pwbrowser.RuntimeName)
	if status != fasthttp.StatusNotFound {
		t.Errorf("a stale revision answered %d", status)
	}
}

// The prefix is reserved, so an unclaimed path inside it is closed here rather
// than reaching application routing — one routing and access rule instead of a
// hole an application could accidentally serve through.
func TestTheReservedNamespaceIsClosed(t *testing.T) {
	for _, path := range []string{pwbrowser.Prefix + "anything", pwbrowser.Prefix} {
		if status, _, body := serve(t, assets(), path); status != fasthttp.StatusNotFound {
			t.Errorf("%s answered %d %q instead of closing the namespace", path, status, body)
		}
	}
	// Everything outside it still reaches the application.
	if _, _, body := serve(t, assets(), "/orders"); body != "application" {
		t.Errorf("an ordinary route was swallowed: %q", body)
	}
}
