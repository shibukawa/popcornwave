package middlewares

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

const testOrigin = "https://app.example"

func csrfChain(t *testing.T, config CSRFConfig) http.Handler {
	t.Helper()
	middleware, err := CSRF(config, session.CookieOptions{Path: "/"}, http.SameSiteLaxMode, nil, nil)
	if err != nil {
		t.Fatalf("CSRF: %v", err)
	}
	return middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
}

func enabledCSRF() CSRFConfig {
	config := DefaultCSRF()
	config.Enabled = true
	return config
}

// withSecret builds a request the session middleware would have annotated.
func withSecret(t *testing.T, r *http.Request, secret string) *http.Request {
	t.Helper()
	return r.WithContext(pwruntime.WithCSRFSecret(r.Context(), secret))
}

func postForm(t *testing.T, path, token string) *http.Request {
	t.Helper()
	body := ""
	if token != "" {
		body = url.Values{"_csrf": {token}}.Encode()
	}
	r := httptest.NewRequest(http.MethodPost, testOrigin+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", testOrigin)
	r.TLS = &tls.ConnectionState{}
	return r
}

func newSecret(t *testing.T) (secret, token string) {
	t.Helper()
	secret, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		t.Fatalf("NewCSRFSecret: %v", err)
	}
	token, err = pwruntime.CSRFToken(secret, nil)
	if err != nil {
		t.Fatalf("CSRFToken: %v", err)
	}
	return secret, token
}

func TestCSRFAcceptsAValidTokenFromEitherChannel(t *testing.T) {
	secret, token := newSecret(t)
	handler := csrfChain(t, enabledCSRF())

	form := withSecret(t, postForm(t, "/orders", token), secret)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, form)
	if w.Code != http.StatusNoContent {
		t.Errorf("hidden field: status = %d, want 204", w.Code)
	}

	// The header is what the runtime sends, and it is read before the body so a
	// fetch never pays for parsing one it does not have.
	header := withSecret(t, postForm(t, "/orders", ""), secret)
	header.Header.Set("X-CSRF-Token", token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, header)
	if w.Code != http.StatusNoContent {
		t.Errorf("header: status = %d, want 204", w.Code)
	}
}

func TestCSRFRefusesMissingWrongAndForeignTokens(t *testing.T) {
	secret, token := newSecret(t)
	_, otherToken := newSecret(t)
	handler := csrfChain(t, enabledCSRF())

	cases := map[string]*http.Request{
		"no token":       withSecret(t, postForm(t, "/orders", ""), secret),
		"another secret": withSecret(t, postForm(t, "/orders", otherToken), secret),
		"malformed":      withSecret(t, postForm(t, "/orders", "not-a-token"), secret),
		"no session":     postForm(t, "/orders", token),
	}
	for name, r := range cases {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", name, w.Code)
		}
	}
}

// A safe method never carries a token, because this protects what changes state
// and a GET that changes state is a defect a token would only hide.
func TestCSRFLeavesSafeMethodsAlone(t *testing.T) {
	handler := csrfChain(t, enabledCSRF())
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		r := httptest.NewRequest(method, testOrigin+"/orders", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", method, w.Code)
		}
	}
}

func TestCSRFAppliesExcludePrecedence(t *testing.T) {
	config := enabledCSRF()
	config.Include = []string{"/admin/**"}
	config.Exclude = []string{"/admin/webhook"}
	handler := csrfChain(t, config)

	// Outside the include set, and inside it but excluded: both pass untouched,
	// with no token and no session.
	for _, path := range []string{"/public/contact", "/admin/webhook"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, postForm(t, path, ""))
		if w.Code != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postForm(t, "/admin/users", ""))
	if w.Code != http.StatusForbidden {
		t.Errorf("/admin/users: status = %d, want 403", w.Code)
	}
}

func TestCSRFChecksOriginBeforeTheToken(t *testing.T) {
	secret, token := newSecret(t)
	config := enabledCSRF()
	config.TrustedOrigins = []string{"https://trusted.example"}
	handler := csrfChain(t, config)

	// A valid token from a foreign origin is still refused: the token check is
	// not the only gate, and an origin the deployment did not name is one.
	foreign := withSecret(t, postForm(t, "/orders", token), secret)
	foreign.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, foreign)
	if w.Code != http.StatusForbidden {
		t.Errorf("foreign origin: status = %d, want 403", w.Code)
	}

	trusted := withSecret(t, postForm(t, "/orders", token), secret)
	trusted.Header.Set("Origin", "https://trusted.example")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, trusted)
	if w.Code != http.StatusNoContent {
		t.Errorf("trusted origin: status = %d, want 204", w.Code)
	}

	// No Origin and no Referer is a refusal rather than a pass. Treating an
	// absent header as trust would make the whole check optional.
	bare := withSecret(t, postForm(t, "/orders", token), secret)
	bare.Header.Del("Origin")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, bare)
	if w.Code != http.StatusForbidden {
		t.Errorf("no origin and no referer: status = %d, want 403", w.Code)
	}

	// Referer is the fallback for a proxy that stripped Origin.
	referer := withSecret(t, postForm(t, "/orders", token), secret)
	referer.Header.Del("Origin")
	referer.Header.Set("Referer", testOrigin+"/orders/new")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, referer)
	if w.Code != http.StatusNoContent {
		t.Errorf("referer fallback: status = %d, want 204", w.Code)
	}
}

// A project that has not turned it on gets exactly what it had before.
func TestCSRFDisabledPassesEverything(t *testing.T) {
	handler := csrfChain(t, DefaultCSRF())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, postForm(t, "/orders", ""))
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestCSRFRejectsAMalformedPatternAtSetup(t *testing.T) {
	config := enabledCSRF()
	config.Include = []string{"no-leading-slash"}
	if _, err := CSRF(config, session.CookieOptions{Path: "/"}, http.SameSiteLaxMode, nil, nil); err == nil {
		t.Fatal("a malformed include pattern was accepted")
	}
}

// TestCSRFBehindATerminatingProxy is the case a deployed application actually
// runs: TLS ends at the proxy, so r.TLS is nil and the request reconstructs an
// http origin while the browser reports https.
//
// Both ways out are exercised here, because a deployment has exactly these two
// and gets no check at all if it has neither.
func TestCSRFBehindATerminatingProxy(t *testing.T) {
	secret, token := newSecret(t)
	// What the proxy actually forwards: no TLS, its own peer address, and the
	// scheme the browser used carried in a header.
	proxied := func() *http.Request {
		r := postForm(t, "/orders", token)
		r.TLS = nil
		r.RemoteAddr = "10.0.0.7:44100"
		r.Header.Set("X-Forwarded-Proto", "https")
		return withSecret(t, r, secret)
	}

	t.Run("refused when neither the proxy nor the origin is declared", func(t *testing.T) {
		response := httptest.NewRecorder()
		csrfChain(t, enabledCSRF()).ServeHTTP(response, proxied())
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want the origin comparison to fail closed", response.Code)
		}
	})

	t.Run("admitted through a declared origin", func(t *testing.T) {
		config := enabledCSRF()
		config.TrustedOrigins = []string{testOrigin}
		response := httptest.NewRecorder()
		csrfChain(t, config).ServeHTTP(response, proxied())
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want a declared origin to be admitted", response.Code)
		}
	})

	t.Run("admitted through a declared proxy", func(t *testing.T) {
		_, network, err := net.ParseCIDR("10.0.0.0/8")
		if err != nil {
			t.Fatal(err)
		}
		middleware, err := CSRF(enabledCSRF(), session.CookieOptions{Path: "/"},
			http.SameSiteLaxMode, nil, []*net.IPNet{network})
		if err != nil {
			t.Fatalf("CSRF: %v", err)
		}
		response := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(response, proxied())
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want the resolved origin to match", response.Code)
		}
	})

	t.Run("an undeclared peer cannot assert the scheme", func(t *testing.T) {
		_, network, err := net.ParseCIDR("10.0.0.0/8")
		if err != nil {
			t.Fatal(err)
		}
		middleware, err := CSRF(enabledCSRF(), session.CookieOptions{Path: "/"},
			http.SameSiteLaxMode, nil, []*net.IPNet{network})
		if err != nil {
			t.Fatalf("CSRF: %v", err)
		}
		request := proxied()
		request.RemoteAddr = "203.0.113.9:44100"
		response := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want a header from outside the trust set ignored", response.Code)
		}
	})
}
