package middlewares

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
)

func resolvedAddress(t *testing.T, networks []*net.IPNet, build func() *http.Request) string {
	t.Helper()
	var seen string
	handler := ResolveClientAddress(networks)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = pwruntime.ClientAddress(r.Context(), r)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), build())
	return seen
}

func forwardedRequest(peer, forwarded string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/orders", nil)
	r.RemoteAddr = peer
	if forwarded != "" {
		r.Header.Set("X-Forwarded-For", forwarded)
	}
	return r
}

func TestResolveClientAddressRecordsTheCaller(t *testing.T) {
	_, network, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	got := resolvedAddress(t, []*net.IPNet{network}, func() *http.Request {
		return forwardedRequest("10.0.0.7:44100", "203.0.113.9, 10.0.0.3")
	})
	if got != "203.0.113.9" {
		t.Fatalf("resolved address = %q, want the caller rather than the proxy", got)
	}
}

// With nothing declared the frame records the peer, which is exactly what the
// callers read directly before it existed.
func TestResolveClientAddressRecordsThePeerWithNothingDeclared(t *testing.T) {
	got := resolvedAddress(t, nil, func() *http.Request {
		return forwardedRequest("10.0.0.7:44100", "203.0.113.9")
	})
	if got != "10.0.0.7" {
		t.Fatalf("resolved address = %q, want the peer", got)
	}
}
