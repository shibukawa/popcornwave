package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// withUpdateSettings publishes the resolution the other runtime would have
// made, which is the seam this half depends on: it reads no configuration file
// of its own.
func withUpdateSettings(t *testing.T) {
	t.Helper()
	previous, had := pwruntime.ResolvedUpdateSettings()
	pwruntime.PublishUpdateSettings(pwruntime.UpdateSettings{
		Enabled:             true,
		ValidatorKey:        "test-validator-key-that-is-long-enough",
		HeaderPrefix:        "Pw",
		DataAttributePrefix: "tb",
		GlobalName:          "popcornwave",
		PathPrefix:          "/_pw/",
		BuildID:             "test-build",
		MaxManifestBytes:    8192,
		CSRFHeaderName:      "X-CSRF-Token",
		CallerOwnsRuntime:   true,
	})
	t.Cleanup(func() {
		if had {
			pwruntime.PublishUpdateSettings(previous)
			return
		}
		pwruntime.PublishUpdateSettings(pwruntime.UpdateSettings{})
	})
}

// serveWith runs one request carrying the headers an update client sends.
func serveWith(t *testing.T, handler fasthttp.RequestHandler, headers map[string]string) (int, string, string) {
	t.Helper()
	var extra strings.Builder
	for name, value := range headers {
		extra.WriteString(name + ": " + value + "\r\n")
	}
	return serveRaw(t, handler, "/orders", extra.String())
}

// An action handler answers with the regions it changed. This is the case the
// whole update surface exists for, and it now runs on this transport.
func TestWriteUpdateAnswersWithTheChangedRegions(t *testing.T) {
	withUpdateSettings(t)
	status, header, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		if !WantsUpdate(r) {
			t.Error("an action request was not recognized as one")
		}
		WriteUpdate(r, fasthttp.StatusOK, Replace("total", staticFragment(`<b id="total">9</b>`)))
	}, map[string]string{"Pw-Render": "action", "Pw-Build": "test-build"})

	if status != fasthttp.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}
	if !strings.Contains(body, "total") {
		t.Errorf("the response does not carry the region: %q", body)
	}
	// The served mode is echoed so a proxy that substituted a body is
	// detectable rather than silently applied.
	if !strings.Contains(header, "Pw-Render") {
		t.Errorf("the response does not echo the served mode:\n%s", header)
	}
}

// A request no update client sent must not be taken as one, or an ordinary
// form post would receive records instead of a page.
func TestWantsUpdateIsFalseForAnOrdinaryRequest(t *testing.T) {
	withUpdateSettings(t)
	_, _, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		if WantsUpdate(r) {
			t.Error("an ordinary request was taken for an update request")
		}
		_, _ = r.WriteString("page")
	}, nil)
	if body != "page" {
		t.Errorf("body = %q", body)
	}
}

// Navigation is the entry that needed nothing from the transport at all: the
// module composes it from the target alone.
func TestWriteUpdateNavigateSendsTheTarget(t *testing.T) {
	withUpdateSettings(t)
	_, _, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		WriteUpdateNavigate(r, "/orders/9")
	}, map[string]string{"Pw-Render": "action", "Pw-Build": "test-build"})
	if !strings.Contains(body, "/orders/9") {
		t.Errorf("the navigation target is not in the response: %q", body)
	}
}

// A target the browser runtime would hand to location.assign must be refused,
// because that call executes a javascript: URL rather than navigating to it.
func TestWriteUpdateNavigateRefusesAScriptTarget(t *testing.T) {
	withUpdateSettings(t)
	status, _, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		WriteUpdateNavigate(r, "javascript:alert(1)")
	}, map[string]string{"Pw-Render": "action", "Pw-Build": "test-build"})
	if status != fasthttp.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
	if strings.Contains(body, "javascript:") {
		t.Errorf("the refused target was repeated in the body: %q", body)
	}
}

// A project that never enabled updates sees the ordinary path rather than a
// failure about a feature it does not use.
func TestUpdateEntriesAreInertWithoutSettings(t *testing.T) {
	pwruntime.PublishUpdateSettings(pwruntime.UpdateSettings{})
	_, _, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		if WantsUpdate(r) {
			t.Error("updates answered while disabled")
		}
		if Redraw(r) {
			t.Error("a redraw answered while disabled")
		}
		_, _ = r.WriteString("page")
	}, map[string]string{"Pw-Render": "action"})
	if body != "page" {
		t.Errorf("body = %q", body)
	}
}

// A navigation request is answered with a delta rather than a document. This is
// the end the whole update path exists for, on this transport.
func TestServeUpdateAnswersANavigationWithADelta(t *testing.T) {
	withUpdateSettings(t)
	leaf := staticFragment(`<h1 id="results">results</h1>`)
	status, header, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		if !ServeUpdate(r, nil, leaf) {
			t.Error("a navigation request was not answered as an update")
			return
		}
	}, map[string]string{"Pw-Render": "navigation", "Pw-Build": "test-build"})

	if status != fasthttp.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}
	// A delta is a record stream, not a document. A cache that handed one to a
	// document request would put records on screen.
	if strings.Contains(strings.ToLower(header), "text/html") {
		t.Errorf("a delta was sent as a document:\n%s", header)
	}
	// Both representations of one URL vary on what selected them, and neither
	// may be stored by a shared cache.
	if !strings.Contains(header, "Vary") {
		t.Errorf("the delta declares no Vary axes:\n%s", header)
	}
	if !strings.Contains(strings.ToLower(header), "no-store") {
		t.Errorf("the delta is missing its cache policy:\n%s", header)
	}
	if body == "" {
		t.Error("the delta carried no records")
	}
}

// An ordinary request must fall through, or every page would answer with
// records to a client that asked for markup.
func TestServeUpdateDeclinesADocumentRequest(t *testing.T) {
	withUpdateSettings(t)
	_, _, body := serveWith(t, func(r *fasthttp.RequestCtx) {
		if ServeUpdate(r, nil, staticFragment(`<p>x</p>`)) {
			t.Error("a document request was answered as an update")
			return
		}
		_, _ = r.WriteString("document")
	}, nil)
	if body != "document" {
		t.Errorf("body = %q", body)
	}
}
