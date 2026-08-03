package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

const testCSRFCookie = "pw_csrf"

func csrfManager(t *testing.T, csrfName string) *Manager[payload] {
	t.Helper()
	manager, err := NewManager[payload](newMapStore(), Options[payload]{
		TTL:    time.Hour,
		Cookie: CookieOptions{Name: "pw_session", Secure: false, HTTPOnly: true, CSRFName: csrfName},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return manager
}

func cookieNamed(response *http.Response, name string) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// The companion cookie is written wherever the session cookie is, so a browser
// holding one always holds the other.
func TestCreateWritesACSRFCookieThatVerifies(t *testing.T) {
	manager := csrfManager(t, testCSRFCookie)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := manager.Create(recorder, request, payload{AccountID: "alice"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	response := recorder.Result()
	if cookieNamed(response, "pw_session") == nil {
		t.Fatal("no session cookie")
	}
	csrf := cookieNamed(response, testCSRFCookie)
	if csrf == nil {
		t.Fatal("no CSRF cookie")
	}
	// The runtime reads it, so it cannot be HttpOnly. That is the one exception
	// to policy:cookie-value-protection and the reason this cookie exists.
	if csrf.HttpOnly {
		t.Error("the CSRF cookie is HttpOnly, so the runtime could not read it")
	}
	if csrf.Value == "" {
		t.Fatal("the CSRF cookie is empty")
	}

	// The value is a masked token rather than the stored secret, and it must
	// verify against the secret the record holds.
	secret := storedSecret(t, manager, response)
	if !pwruntime.VerifyCSRFToken(secret, csrf.Value) {
		t.Error("the cookie value does not verify against the session secret")
	}
	if csrf.Value == secret {
		t.Error("the cookie carries the bare secret rather than a derived token")
	}
}

// storedSecret reads the secret the manager wrote, through the middleware that
// is the only thing entitled to see it.
func storedSecret(t *testing.T, manager *Manager[payload], response *http.Response) string {
	t.Helper()
	next := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range response.Cookies() {
		next.AddCookie(cookie)
	}
	var secret string
	handler := manager.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		secret, _ = pwruntime.CSRFSecret(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), next)
	if secret == "" {
		t.Fatal("the session carried no CSRF secret")
	}
	return secret
}

// Rotation replaces the secret, so the cookie a page loaded before a login
// cannot be presented after one.
func TestRotateReplacesTheCSRFCookie(t *testing.T) {
	manager := csrfManager(t, testCSRFCookie)
	first := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := manager.Create(first, request, payload{AccountID: "alice"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before := cookieNamed(first.Result(), testCSRFCookie).Value

	rotated := httptest.NewRecorder()
	next := httptest.NewRequest(http.MethodPost, "/login", nil)
	for _, cookie := range first.Result().Cookies() {
		next.AddCookie(cookie)
	}
	if err := manager.Rotate(rotated, next, payload{AccountID: "alice"}); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	after := cookieNamed(rotated.Result(), testCSRFCookie)
	if after == nil {
		t.Fatal("rotation wrote no CSRF cookie")
	}
	if after.Value == before {
		t.Error("rotation reused the previous token")
	}
	// The old token belonged to the revoked session, so it must not verify
	// against the new one.
	secret := storedSecret(t, manager, rotated.Result())
	if pwruntime.VerifyCSRFToken(secret, before) {
		t.Error("a token minted before the rotation still verifies after it")
	}
}

func TestDeleteClearsTheCSRFCookie(t *testing.T) {
	manager := csrfManager(t, testCSRFCookie)
	created := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := manager.Create(created, request, payload{AccountID: "alice"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	recorder := httptest.NewRecorder()
	next := httptest.NewRequest(http.MethodPost, "/logout", nil)
	for _, cookie := range created.Result().Cookies() {
		next.AddCookie(cookie)
	}
	if err := manager.Delete(recorder, next); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	csrf := cookieNamed(recorder.Result(), testCSRFCookie)
	if csrf == nil {
		t.Fatal("logout wrote no CSRF cookie header")
	}
	if csrf.Value != "" || csrf.MaxAge >= 0 {
		t.Errorf("logout did not expire the CSRF cookie: value=%q maxAge=%d", csrf.Value, csrf.MaxAge)
	}
}

// A deployment that verifies no token is handed none, so adopting this costs a
// project with the check off exactly nothing.
func TestNoCSRFNameWritesNoCompanionCookie(t *testing.T) {
	manager := csrfManager(t, "")
	recorder := httptest.NewRecorder()
	if err := manager.Create(recorder, httptest.NewRequest(http.MethodPost, "/login", nil), payload{AccountID: "alice"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	response := recorder.Result()
	if cookieNamed(response, "pw_session") == nil {
		t.Fatal("no session cookie")
	}
	if cookieNamed(response, testCSRFCookie) != nil {
		t.Error("a CSRF cookie was written with no name configured")
	}
}
