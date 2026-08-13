package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func corsRequest(method, target, origin string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request
}

// A preflight is answered here and nowhere else. It carries no cookie, no
// Authorization and no token, so every frame below would read it as a caller
// asking for something it may not have — and a 401 to a preflight is a request
// the browser never sends.
func TestCORSPreflightIsAnsweredWithoutReachingTheHandler(t *testing.T) {
	reached := false
	handler := corsFrame(t, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))

	request := corsRequest(http.MethodOptions, "/api/things", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", "POST")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if reached {
		t.Fatal("the preflight reached the handler")
	}
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow-origin = %q", got)
	}
	// The policy headers go out on it too, because this is the same frame.
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("the preflight answer carried no policy headers")
	}
}

// An OPTIONS that is not a preflight belongs to whatever else answers OPTIONS,
// which for this framework is the router, deliberately answering none.
func TestPlainOptionsReachesTheHandler(t *testing.T) {
	reached := false
	handler := corsFrame(t, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	handler.ServeHTTP(httptest.NewRecorder(), corsRequest(http.MethodOptions, "/api/things", "https://app.example.com"))
	if !reached {
		t.Fatal("a plain OPTIONS was swallowed")
	}
}

// The whole reason the frame sits where it does. A refusal written by any frame
// below carries the marking, so the caller reads the status the framework
// computed instead of an opaque network error.
func TestTheMarkingSurvivesEveryRefusalBelow(t *testing.T) {
	for name, refuse := range map[string]http.HandlerFunc{
		"unauthorized": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
		"rate limited": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		},
		"panicked": func(http.ResponseWriter, *http.Request) { panic("boom") },
	} {
		t.Run(name, func(t *testing.T) {
			// The recover frame is outside this one, so a panic's 500 is
			// written after the marking frame returned. It is still marked,
			// because the header map is one map for the whole chain.
			handler := Recover(func(w http.ResponseWriter, _ *http.Request, _ error) {
				w.WriteHeader(http.StatusInternalServerError)
			})(corsFrame(t, nil)(refuse))

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, corsRequest(http.MethodGet, "/api/things", "https://app.example.com"))

			if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
				t.Fatalf("status %d reached the caller unmarked: allow-origin = %q", response.Code, got)
			}
		})
	}
}

// Without the expose header a cross-origin client cannot read the retry
// metadata on the 429 it was meant to act on, and keeps retrying at the rate
// that limited it.
func TestTheRetryMetadataIsExposed(t *testing.T) {
	handler := corsFrame(t, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, corsRequest(http.MethodGet, "/api/things", "https://app.example.com"))
	exposed := response.Header().Get("Access-Control-Expose-Headers")
	for _, want := range []string{"Retry-After", "X-RateLimit-Remaining"} {
		if !strings.Contains(exposed, want) {
			t.Errorf("expose-headers %q omits %s", exposed, want)
		}
	}
}

// Vary is added rather than set, so a value another frame put there survives.
func TestVaryIsAddedRatherThanReplaced(t *testing.T) {
	handler := corsFrame(t, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, corsRequest(http.MethodGet, "/api/things", ""))
	values := response.Header().Values("Vary")
	if len(values) != 2 || values[0] != "Origin" || values[1] != "Accept-Encoding" {
		t.Fatalf("vary = %v", values)
	}
}

// A deployment that turned the headers off and admits an origin gets the
// admission and not the headers back.
func TestCORSAloneInstallsNoPolicyHeaders(t *testing.T) {
	config := DefaultCORS()
	config.Enabled = true
	config.AllowedOrigins = []string{"https://app.example.com"}
	middleware, err := SecurityHeaders(SecurityHeadersConfig{}, WithCORS(config, "X-CSRF-Token"))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(response, corsRequest(http.MethodGet, "/api/things", "https://app.example.com"))

	if response.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("the origin was not admitted")
	}
	if got := response.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("a disabled header policy still sent %q", got)
	}
}

// A misconfiguration is an error before the port is bound rather than a wrong
// header per request.
func TestAMisconfiguredPolicyFailsConstruction(t *testing.T) {
	config := DefaultCORS()
	config.Enabled = true
	if _, err := SecurityHeaders(DefaultSecurityHeaders(), WithCORS(config, "X-CSRF-Token")); err == nil {
		t.Fatal("a policy admitting no origin was accepted")
	}
}
