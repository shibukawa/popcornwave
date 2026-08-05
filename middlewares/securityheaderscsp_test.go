package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func servedHeaders(t *testing.T, config SecurityHeadersConfig) http.Header {
	t.Helper()
	middleware, err := SecurityHeaders(config)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	return recorder.Header()
}

// A project that names no policy still gets one. It used to get none, which put
// nothing between an HTML-injection sink and script running on the origin — and
// the CSRF companion cookie is readable by script by design, so script on the
// origin can mint a valid token.
func TestAProjectThatNamesNoPolicyStillGetsOne(t *testing.T) {
	policy := servedHeaders(t, DefaultSecurityHeaders()).Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("the shipped defaults sent no Content-Security-Policy")
	}
	for _, directive := range []string{
		"script-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(policy, directive) {
			t.Errorf("the default policy %q is missing %q", policy, directive)
		}
	}
	// The directives an ordinary page needs are deliberately absent, so an
	// application serving images or fonts from elsewhere keeps working without
	// editing configuration.
	for _, directive := range []string{"default-src", "img-src", "font-src", "style-src", "connect-src"} {
		if strings.Contains(policy, directive) {
			t.Errorf("the default policy restricts %s, which breaks ordinary pages", directive)
		}
	}
}

func TestAConfiguredPolicyReplacesTheDefault(t *testing.T) {
	config := DefaultSecurityHeaders()
	config.ContentSecurityPolicy = "default-src 'self'"
	if got := servedHeaders(t, config).Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("Content-Security-Policy = %q, want the configured value", got)
	}
}

// Empty now means the default, so a project that wants no policy at all needs a
// way to say so that is not silence.
func TestOffSendsNoPolicy(t *testing.T) {
	for _, value := range []string{"off", "OFF", "  off  "} {
		config := DefaultSecurityHeaders()
		config.ContentSecurityPolicy = value
		if got := servedHeaders(t, config).Get("Content-Security-Policy"); got != "" {
			t.Errorf("%q sent Content-Security-Policy = %q, want none", value, got)
		}
	}
}

func TestReportOnlyStaysAbsentUnlessAsked(t *testing.T) {
	if got := servedHeaders(t, DefaultSecurityHeaders()).Get("Content-Security-Policy-Report-Only"); got != "" {
		t.Errorf("Content-Security-Policy-Report-Only = %q, want none by default", got)
	}
	config := DefaultSecurityHeaders()
	config.ContentSecurityPolicyReportOnly = "default-src 'self'"
	if got := servedHeaders(t, config).Get("Content-Security-Policy-Report-Only"); got != "default-src 'self'" {
		t.Errorf("Content-Security-Policy-Report-Only = %q, want the configured value", got)
	}
}
