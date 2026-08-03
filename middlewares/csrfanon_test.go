package middlewares

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

func anonSecret(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func anonConfig(t *testing.T) CSRFConfig {
	t.Helper()
	config := enabledCSRF()
	config.Anonymous = AnonymousCSRFConfig{Enabled: true, Secret: anonSecret(t)}
	return config
}

// observed captures the secret the chain established, so a test can mint a
// token for it exactly as a rendered page would.
func anonChain(t *testing.T, config CSRFConfig, seen *string) http.Handler {
	t.Helper()
	middleware, err := CSRF(config, session.CookieOptions{Path: "/"}, http.SameSiteLaxMode, nil)
	if err != nil {
		t.Fatalf("CSRF: %v", err)
	}
	return middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret, ok := pwruntime.CSRFSecret(r.Context()); ok && seen != nil {
			*seen = secret
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

// A GET is what renders the form, so the secret has to exist by then rather
// than being issued when the post arrives.
func TestAnonymousCSRFIssuesOnASafeRequest(t *testing.T) {
	var secret string
	handler := anonChain(t, anonConfig(t), &secret)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, testOrigin+"/contact", nil))

	if secret == "" {
		t.Fatal("a GET established no secret, so a form could not have rendered one")
	}
	response := recorder.Result()
	signed := cookieByName(response, "pw_csrf_anon")
	if signed == nil {
		t.Fatal("no signed secret cookie")
	}
	// The secret is never read by script; only the token beside it is.
	if !signed.HttpOnly {
		t.Error("the signed secret cookie is readable by script")
	}
	runtimeCookie := cookieByName(response, pwruntime.CSRFCookieName)
	if runtimeCookie == nil {
		t.Fatal("no runtime token cookie")
	}
	if runtimeCookie.HttpOnly {
		t.Error("the runtime cookie is HttpOnly, so the runtime could not read it")
	}
	if !pwruntime.VerifyCSRFToken(secret, runtimeCookie.Value) {
		t.Error("the runtime cookie does not verify against the issued secret")
	}
}

// The whole point of the signed cookie: no session record is written, so an
// anonymous population costs nothing to remember.
func TestAnonymousCSRFAcceptsAPostWithNoSession(t *testing.T) {
	var secret string
	config := anonConfig(t)
	handler := anonChain(t, config, &secret)

	issued := httptest.NewRecorder()
	handler.ServeHTTP(issued, httptest.NewRequest(http.MethodGet, testOrigin+"/contact", nil))
	if secret == "" {
		t.Fatal("no secret issued")
	}
	token, err := pwruntime.CSRFToken(secret, nil)
	if err != nil {
		t.Fatalf("CSRFToken: %v", err)
	}

	post := postForm(t, "/contact", token)
	for _, cookie := range issued.Result().Cookies() {
		post.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, post)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", recorder.Code)
	}
}

// A cookie the server did not sign cannot be presented. Without the signature
// this would be the naive double-submit shape, where anyone who can set a
// cookie satisfies the check.
func TestAnonymousCSRFRefusesAnUnsignedCookie(t *testing.T) {
	handler := anonChain(t, anonConfig(t), nil)
	forged, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		t.Fatalf("NewCSRFSecret: %v", err)
	}
	token, err := pwruntime.CSRFToken(forged, nil)
	if err != nil {
		t.Fatalf("CSRFToken: %v", err)
	}
	post := postForm(t, "/contact", token)
	post.AddCookie(&http.Cookie{Name: "pw_csrf_anon", Value: forged})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, post)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", recorder.Code)
	}
}

// A session secret wins, so logging in does not leave the anonymous one in
// play. The two never both decide.
func TestASessionSecretTakesPrecedenceOverTheAnonymousOne(t *testing.T) {
	var secret string
	handler := anonChain(t, anonConfig(t), &secret)
	sessionSecret, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		t.Fatalf("NewCSRFSecret: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, testOrigin+"/dashboard", nil)
	request = request.WithContext(pwruntime.WithCSRFSecret(request.Context(), sessionSecret))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if secret != sessionSecret {
		t.Errorf("the anonymous issuer replaced the session secret")
	}
	if cookieByName(recorder.Result(), "pw_csrf_anon") != nil {
		t.Error("an anonymous cookie was issued to a request that had a session")
	}
}

// Left off, a visitor with no session still has no secret, so an unsafe form on
// a public page is refused rather than quietly protected by nothing.
func TestAnonymousDisabledLeavesAPublicPostRefused(t *testing.T) {
	handler := anonChain(t, enabledCSRF(), nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, postForm(t, "/contact", ""))
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", recorder.Code)
	}
}

func TestAnonymousCSRFRequiresASigningSecret(t *testing.T) {
	config := enabledCSRF()
	config.Anonymous = AnonymousCSRFConfig{Enabled: true}
	if _, err := CSRF(config, session.CookieOptions{Path: "/"}, http.SameSiteLaxMode, nil); err == nil {
		t.Fatal("the anonymous path was enabled with no signing secret")
	}
}

func cookieByName(response *http.Response, name string) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
