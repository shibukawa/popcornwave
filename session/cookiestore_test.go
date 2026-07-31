package session

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// cookieStore returns the browser backend under the typed adapter a Manager
// takes, which is exactly how a host uses it.
func cookieStore(t *testing.T, keys *Keyring, now func() time.Time) Store[payload] {
	t.Helper()
	return Typed[payload](rawCookieStore(t, keys, now), JSONCodec[payload]{})
}

func rawCookieStore(t *testing.T, keys *Keyring, now func() time.Time) *CookieStore {
	t.Helper()
	store, err := NewCookieStore(CookieStoreOptions{
		Keys:   keys,
		Cookie: CookieOptions{Secure: true, HTTPOnly: true},
		Now:    now,
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	return store
}

// browser carries the cookies of one client between requests.
type browser struct {
	cookies map[string]*http.Cookie
}

func newBrowser() *browser { return &browser{cookies: make(map[string]*http.Cookie)} }

// accept applies the Set-Cookie headers of a response the way a browser would.
func (b *browser) accept(recorder *httptest.ResponseRecorder) {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.MaxAge < 0 || cookie.Value == "" {
			delete(b.cookies, cookie.Name)
			continue
		}
		b.cookies[cookie.Name] = cookie
	}
}

func (b *browser) request() *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range b.cookies {
		request.AddCookie(cookie)
	}
	return request
}

func (b *browser) copy() *browser {
	clone := newBrowser()
	for name, cookie := range b.cookies {
		copied := *cookie
		clone.cookies[name] = &copied
	}
	return clone
}

// login creates a session for account and hands the resulting cookies to a
// browser.
func login(t *testing.T, manager *Manager[payload], account string) *browser {
	t.Helper()
	recorder := httptest.NewRecorder()
	if err := manager.Create(recorder, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: account}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	client := newBrowser()
	client.accept(recorder)
	return client
}

// resolve runs one request of client through the session middleware and
// reports the session the handler saw.
func resolve(t *testing.T, manager *Manager[payload], client *browser) (View[payload], bool, *httptest.ResponseRecorder) {
	t.Helper()
	var seen View[payload]
	var found bool
	handler := manager.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, found = Read[payload](r.Context())
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, client.request())
	client.accept(recorder)
	return seen, found, recorder
}

func TestCookieStoreCarriesASessionWithoutABackend(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	keys := testKeyring(t, 1)
	manager := testManager(t, cookieStore(t, keys, c.Now), defaultOptions(c.Now))

	client := login(t, manager, "account-1")
	if client.cookies[DefaultCookieName] == nil || client.cookies[DefaultDataCookieName] == nil {
		t.Fatalf("cookies = %#v", client.cookies)
	}
	record := client.cookies[DefaultDataCookieName]
	if !record.Secure || !record.HttpOnly {
		t.Fatalf("record cookie policy = %#v", record)
	}
	if strings.Contains(record.Value, "account-1") {
		t.Fatal("record cookie leaks its payload")
	}

	seen, found, _ := resolve(t, manager, client)
	if !found || seen.Data.AccountID != "account-1" || seen.Method != "test" {
		t.Fatalf("resolved session = %#v found=%v", seen, found)
	}

	// The session is in the cookies, not in this process: another process
	// holding the same keyring resolves the same request.
	other := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))
	seen, found, _ = resolve(t, other, client)
	if !found || seen.Data.AccountID != "account-1" {
		t.Fatalf("second process session = %#v found=%v", seen, found)
	}
}

func TestCookieStoreRejectsARecordFromAnotherSession(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))

	first := login(t, manager, "account-1")
	second := login(t, manager, "account-2")

	// The record is sealed under the hash of its own token, so pairing one
	// client's record with another client's token does not authenticate
	// either of them.
	forged := newBrowser()
	forged.cookies[DefaultCookieName] = second.cookies[DefaultCookieName]
	forged.cookies[DefaultDataCookieName] = first.cookies[DefaultDataCookieName]
	if _, found, _ := resolve(t, manager, forged); found {
		t.Fatal("a record was accepted under a foreign token")
	}
}

func TestCookieStoreRefusesTamperedAndForeignRecords(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))
	client := login(t, manager, "account-1")

	tampered := client.copy()
	record := *tampered.cookies[DefaultDataCookieName]
	if record.Value[4] == 'A' {
		record.Value = "1.e.B" + record.Value[5:]
	} else {
		record.Value = "1.e.A" + record.Value[5:]
	}
	tampered.cookies[DefaultDataCookieName] = &record
	_, found, recorder := resolve(t, manager, tampered)
	if found {
		t.Fatal("a tampered record was accepted")
	}
	// Stale browser state is cleared rather than reported as a failure.
	if cleared := sessionCookie(t, recorder, DefaultCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("session cookie was not cleared: %#v", cleared)
	}

	// A record written under another keyring is not readable here either.
	foreign := testManager(t, cookieStore(t, testKeyring(t, 9), c.Now), defaultOptions(c.Now))
	if _, found, _ := resolve(t, foreign, client); found {
		t.Fatal("a record sealed under another keyring was accepted")
	}
}

func TestCookieStoreRenewsAndExpiresLikeABackend(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))
	client := login(t, manager, "account-1")
	issued := client.cookies[DefaultDataCookieName].Value

	// Before the renewal interval the record cookie is left alone.
	c.advance(time.Minute)
	if _, _, recorder := resolve(t, manager, client); sessionCookie(t, recorder, DefaultDataCookieName) != nil {
		t.Fatal("record cookie rewritten before the renewal interval")
	}
	// After it, idle expiry moves and the browser gets the new record.
	c.advance(10 * time.Minute)
	seen, found, _ := resolve(t, manager, client)
	if !found {
		t.Fatal("session lost during renewal")
	}
	if client.cookies[DefaultDataCookieName].Value == issued {
		t.Fatal("record cookie was not renewed")
	}
	if !seen.IdleExpiresAt.After(c.Now()) {
		t.Fatalf("idle expiry = %v", seen.IdleExpiresAt)
	}

	// Past the absolute lifetime the sealed stamp ends the session even though
	// the client still holds both cookies.
	kept := client.copy()
	c.advance(2 * time.Hour)
	if _, found, _ := resolve(t, manager, kept); found {
		t.Fatal("an expired session was accepted")
	}
}

func TestCookieStoreRotateReplacesBothCookies(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))
	client := login(t, manager, "account-1")
	before := *client.cookies[DefaultCookieName]

	recorder := httptest.NewRecorder()
	if err := manager.Rotate(recorder, client.request(), payload{AccountID: "account-1"}); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	client.accept(recorder)
	if client.cookies[DefaultCookieName].Value == before.Value {
		t.Fatal("rotation reused the token")
	}
	seen, found, _ := resolve(t, manager, client)
	if !found || seen.Data.AccountID != "account-1" {
		t.Fatalf("session after rotation = %#v found=%v", seen, found)
	}
}

func TestCookieStoreDeleteClearsTheBrowserButCannotRevoke(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))
	client := login(t, manager, "account-1")
	kept := client.copy()

	recorder := httptest.NewRecorder()
	if err := manager.Delete(recorder, client.request()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	client.accept(recorder)
	if len(client.cookies) != 0 {
		t.Fatalf("cookies after Delete = %#v", client.cookies)
	}
	if _, found, _ := resolve(t, manager, client); found {
		t.Fatal("session survived in the browser that logged out")
	}

	// The documented limit of a client-side store: a copy taken before the
	// logout stays valid until its sealed expiry, because there is no server
	// record to revoke. A deployment that must end sessions on demand uses a
	// backend store.
	if _, found, _ := resolve(t, manager, kept); !found {
		t.Fatal("expectation changed: a retained copy is now revoked")
	}
}

func TestCookieStoreRefusesARecordTheBrowserWouldDrop(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, cookieStore(t, testKeyring(t, 1), c.Now), defaultOptions(c.Now))

	err := manager.Create(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/", nil),
		payload{AccountID: strings.Repeat("x", DefaultMaxCookieBytes)},
	)
	if !errors.Is(err, ErrCookieTooLarge) {
		t.Fatalf("oversized session error = %v", err)
	}
}

func TestCookieStoreReportsUseOutsideARequest(t *testing.T) {
	store := rawCookieStore(t, testKeyring(t, 1), nil)
	// Without the Manager binding a request there is no cookie to read, which
	// is a programming error rather than a missing session.
	if _, err := store.Get(t.Context(), strings.Repeat("a", 64)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound Get error = %v", err)
	}
	if err := store.Put(t.Context(), strings.Repeat("a", 64), RawRecord{Payload: []byte("{}")}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound Put error = %v", err)
	}
	if _, err := store.Get(t.Context(), "not-a-key-hash"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("malformed key error = %v", err)
	}
}

func TestCookieStoreRequiresAKeyring(t *testing.T) {
	if _, err := NewCookieStore(CookieStoreOptions{}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("keyless store error = %v", err)
	}
}
