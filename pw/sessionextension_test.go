package pw

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/popcornwave/sessionconfig"
)

type visitLocale struct {
	Tag string `json:"tag"`
}

func replaceConfigForTest[T any](t *testing.T, value T) *T {
	t.Helper()
	key := reflect.TypeFor[T]()
	configState.Lock()
	previous, existed := configState.entries[key]
	configState.entries[key] = configEntry{ptr: &value}
	configState.Unlock()
	t.Cleanup(func() {
		configState.Lock()
		if existed {
			configState.entries[key] = previous
		} else {
			delete(configState.entries, key)
		}
		configState.Unlock()
	})
	return &value
}

func TestDevelopmentSessionModesResolveToOneIntendedStore(t *testing.T) {
	restoreEnvState(t)
	setEnv(EnvDevelopment, true)
	t.Cleanup(func() { _ = closeSession(context.Background()) })
	replaceConfigForTest(t, SecurityConfig{CSRF: CSRFConfig{Enabled: true}})
	config := replaceConfigForTest(t, SessionConfig{
		Enabled:     true,
		Backend:     SessionBackendDevVolatile,
		Retention:   time.Hour,
		Cookie:      SessionCookieConfig{Name: "pw_session", Path: "/", SameSite: "lax"},
		CookieStore: SessionCookieStoreConfig{Name: session.DefaultDataCookieName},
	})

	serve := func(t *testing.T, middleware Middleware) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			handle, ok := session.Value[middlewares.CSRFSecret](r.Context())
			if !ok {
				t.Fatal("CSRF session slot was not registered")
			}
			if err := handle.Set(middlewares.CSRFSecret{Secret: "test"}); err != nil {
				t.Fatal(err)
			}
		})).ServeHTTP(recorder, request)
		return recorder
	}

	volatile, err := setupSession(context.Background())
	if err != nil {
		t.Fatalf("dev-volatile setup: %v", err)
	}
	volatileResponse := serve(t, volatile)
	if cookieOfName(volatileResponse, session.DefaultCookieName) == nil {
		t.Fatal("dev-volatile emitted no opaque session token")
	}
	if cookieOfName(volatileResponse, session.DefaultDataCookieName) != nil {
		t.Fatal("dev-volatile emitted a sealed record cookie")
	}

	config.Backend = SessionBackendDevPersist
	if _, err := setupSession(context.Background()); err == nil {
		t.Fatal("dev-persist started without a stable keyring")
	}
	config.Keyring.Secret = base64.StdEncoding.EncodeToString(make([]byte, 32))
	persist, err := setupSession(context.Background())
	if err != nil {
		t.Fatalf("dev-persist setup: %v", err)
	}
	persistResponse := serve(t, persist)
	if cookieOfName(persistResponse, session.DefaultDataCookieName) == nil {
		t.Fatal("dev-persist emitted no sealed record cookie")
	}
}

func cookieOfName(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
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

// The record lifetime is the shorter of two ceilings on different things: how
// long the store may hold bytes, and how long a proof of identity stays good.
func TestTheRecordLifetimeIsTheShorterCeiling(t *testing.T) {
	const day = 24 * time.Hour
	for name, c := range map[string]struct {
		retention, ttl, want time.Duration
	}{
		"auth narrows the store":   {30 * day, day, day},
		"the store narrows auth":   {day, 30 * day, day},
		"no auth leaves the store": {30 * day, 0, 30 * day},
		"no store leaves auth":     {0, day, day},
		"neither bounds it":        {0, 0, 0},
	} {
		if got := recordLifetime(c.retention, c.ttl); got != c.want {
			t.Errorf("%s: recordLifetime(%v, %v) = %v, want %v", name, c.retention, c.ttl, got, c.want)
		}
	}
}

// A server record with no deadline is one the sweep has no cutoff for, and the
// store reads a zero expiry as already past. Refusing at startup is what keeps
// that from looking like a session that silently never persists.
func TestAServerBackendRefusesAnUnboundedRecord(t *testing.T) {
	config := SessionConfig{
		Enabled:   true,
		Backend:   SessionBackendRDB,
		Retention: 0,
		Cookie:    SessionCookieConfig{Name: "pw_session", Path: "/", SameSite: "lax"},
	}
	options, err := sessionOptions(config, sessionconfig.SessionLifetimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if options.TTL > 0 {
		t.Fatalf("TTL = %v, want none so the refusal has something to catch", options.TTL)
	}
	// The cookie backend keeps nothing on a server, so it is not refused.
	config.Backend = SessionBackendCookie
	if _, err := sessionOptions(config, sessionconfig.SessionLifetimeConfig{}); err != nil {
		t.Fatalf("the cookie backend was refused an unbounded record: %v", err)
	}
}
