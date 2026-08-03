package pw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/popcornwave/sessionconfig"
)

type visitLocale struct {
	Tag string `json:"tag"`
}

// Session storage does not depend on a login. An application that imports no
// authentication plugin still declares slots, still gets the middleware, and
// still reads its state back.
//
// The dependency runs one way: pw installs the session and plugin/auth drives
// it. Nothing here can import plugin/auth, which is what makes that true rather
// than merely intended.
func TestSessionStorageServesAnApplicationWithNoAuthentication(t *testing.T) {
	registry := session.NewRegistry()
	if err := session.Register[visitLocale](registry, "locale", session.ReadOnly, nil,
		session.OutlivesSession(session.BrowserMax)); err != nil {
		t.Fatal(err)
	}
	keys, err := session.ParseKeyring("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatal(err)
	}
	// No lifetime source: the zero value is what Config returns for a binding
	// no linked plugin registered, and it means bounded by the browser alone.
	options, err := sessionOptions(SessionConfig{
		Enabled: true,
		Backend: SessionBackendCookie,
		Cookie:  SessionCookieConfig{Name: "pw_session", Path: "/", SameSite: "lax"},
		Keyring: SessionKeyringConfig{},
	}, sessionconfig.SessionLifetimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	options.Keys = keys
	manager, err := session.NewManager(registry, nil, options)
	if err != nil {
		t.Fatalf("a registry with no authentication was refused: %v", err)
	}

	write := httptest.NewRecorder()
	manager.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handle, ok := session.Value[visitLocale](r.Context())
		if !ok {
			t.Fatal("no slot handle without authentication")
		}
		if err := handle.Set(visitLocale{Tag: "ja"}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	})).ServeHTTP(write, httptest.NewRequest(http.MethodGet, "/", nil))

	read := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range write.Result().Cookies() {
		read.AddCookie(cookie)
	}
	manager.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		value, ok := session.Load[visitLocale](r.Context())
		if !ok || value.Tag != "ja" {
			t.Fatalf("locale = %#v ok=%v", value, ok)
		}
	})).ServeHTTP(httptest.NewRecorder(), read)
}

// With no lifetime source the session is bounded by the browser: the token
// cookie carries no Max-Age and no absolute deadline is stamped.
func TestNoLifetimeSourceLeavesTheSessionToTheBrowser(t *testing.T) {
	options, err := sessionOptions(SessionConfig{
		Enabled: true,
		Backend: SessionBackendCookie,
		Cookie:  SessionCookieConfig{Name: "pw_session", Path: "/", SameSite: "lax"},
	}, sessionconfig.SessionLifetimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if options.TTL != 0 || options.IdleTimeout != 0 {
		t.Fatalf("options = %+v, want no declared lifetime", options)
	}
}
