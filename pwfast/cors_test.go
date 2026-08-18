package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

func corsFrame(t *testing.T, mutate func(*CORSConfig)) Middleware {
	t.Helper()
	config := DefaultCORS()
	config.Enabled = true
	config.AllowedOrigins = []string{"https://app.example.com"}
	if mutate != nil {
		mutate(&config)
	}
	middleware, err := SecurityHeaders(DefaultSecurityHeaders(), WithCORS(config, "X-CSRF-Token"))
	if err != nil {
		t.Fatal(err)
	}
	return middleware
}

func TestCORSPreflightIsAnsweredHere(t *testing.T) {
	reached := false
	handler := Chain(func(*fasthttp.RequestCtx) { reached = true }, corsFrame(t, nil))
	status, header, _ := serveRequest(t, handler, "OPTIONS", "/api/things",
		"Origin: https://app.example.com\r\nAccess-Control-Request-Method: POST\r\n", "")

	if reached {
		t.Fatal("the preflight reached the handler")
	}
	if status != fasthttp.StatusNoContent {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(header, "Access-Control-Allow-Origin: https://app.example.com") {
		t.Fatalf("header:\n%s", header)
	}
}

func TestCORSMarkingReachesARefusalBelow(t *testing.T) {
	handler := Chain(func(r *fasthttp.RequestCtx) {
		r.Response.SetStatusCode(fasthttp.StatusTooManyRequests)
	}, corsFrame(t, nil))
	status, header, _ := serveRaw(t, handler, "/api/things", "Origin: https://app.example.com\r\n")

	if status != fasthttp.StatusTooManyRequests {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(header, "Access-Control-Allow-Origin: https://app.example.com") {
		t.Fatalf("a 429 reached the caller unmarked:\n%s", header)
	}
	if !strings.Contains(header, "Access-Control-Expose-Headers") {
		t.Fatalf("no retry metadata was exposed:\n%s", header)
	}
}

func TestCORSUnlistedOriginIsServedUnmarked(t *testing.T) {
	handler := Chain(func(r *fasthttp.RequestCtx) { r.SetStatusCode(fasthttp.StatusOK) }, corsFrame(t, nil))
	status, header, _ := serveRaw(t, handler, "/api/things", "Origin: https://evil.example.com\r\n")

	if status != fasthttp.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if strings.Contains(header, "Access-Control-Allow-Origin") {
		t.Fatalf("an unlisted origin was marked:\n%s", header)
	}
	// It still varies, so a shared cache cannot hand this unmarked response to
	// the origin that would have been marked.
	if !strings.Contains(header, "Vary: Origin") {
		t.Fatalf("no Vary:\n%s", header)
	}
}

// The decision is the shared leaf's, so the two transports cannot disagree
// about it. This asserts that rather than asserting one header list twice.
func TestTheDecisionIsTheOneTheSharedLeafMade(t *testing.T) {
	config := DefaultCORS()
	config.Enabled = true
	config.AllowedOrigins = []string{"https://app.example.com"}
	resolved, err := pwruntime.ResolveCORS(config, "X-CSRF-Token")
	if err != nil {
		t.Fatal(err)
	}
	decision := resolved.Decide("/api/things", "", "GET", "https://app.example.com", "", "")

	handler := Chain(func(r *fasthttp.RequestCtx) {}, corsFrame(t, nil))
	_, header, _ := serveRaw(t, handler, "/api/things", "Origin: https://app.example.com\r\n")
	for _, entry := range decision.Headers {
		if !strings.Contains(header, entry.Name+": "+entry.Value) {
			t.Errorf("the transport did not send %s: %s\n%s", entry.Name, entry.Value, header)
		}
	}
}
