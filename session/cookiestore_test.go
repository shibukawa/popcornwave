package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// cookieManager is a deployment that selected the cookie backend: nothing is
// kept on the server, so the sealed record cookie is the whole session.
func cookieManager(t *testing.T, keys *Keyring, now func() time.Time) *Manager {
	t.Helper()
	options := Options{
		TTL:             time.Hour,
		IdleTimeout:     30 * time.Minute,
		RenewalInterval: 5 * time.Minute,
		Cookie:          CookieOptions{Secure: true, HTTPOnly: true},
		Keys:            keys,
		Now:             now,
	}
	manager, err := NewManager(accountRegistry(t), nil, options)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return manager
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

func (b *browser) list() []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(b.cookies))
	for _, cookie := range b.cookies {
		cookies = append(cookies, cookie)
	}
	return cookies
}

func (b *browser) copy() *browser {
	clone := newBrowser()
	for name, cookie := range b.cookies {
		copied := *cookie
		clone.cookies[name] = &copied
	}
	return clone
}

// login writes the account slot and rotates, which is what a completed sign-in
// does, and hands the resulting cookies to a browser.
func login(t *testing.T, manager *Manager, client *browser, account string) *browser {
	t.Helper()
	recorder := run(manager, client.list(), func(w http.ResponseWriter, r *http.Request) {
		handle, ok := Value[payload](r.Context())
		if !ok {
			t.Fatal("no slot handle")
		}
		if err := handle.Set(payload{AccountID: account}); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := manager.Rotate(w, r); err != nil {
			t.Fatalf("Rotate: %v", err)
		}
	})
	client.accept(recorder)
	return client
}

// resolve runs one request of client through the session middleware and reports
// the session the handler saw.
func resolve(t *testing.T, manager *Manager, client *browser) (payload, bool, *httptest.ResponseRecorder) {
	t.Helper()
	var seen payload
	var found bool
	recorder := run(manager, client.list(), func(_ http.ResponseWriter, r *http.Request) {
		seen, found = Load[payload](r.Context())
	})
	client.accept(recorder)
	return seen, found, recorder
}

func TestCookieStoreCarriesASessionWithoutABackend(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := cookieManager(t, testKeyring(t, 1), c.Now)

	client := login(t, manager, newBrowser(), "account-1")
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
	if !found || seen.AccountID != "account-1" {
		t.Fatalf("resolved session = %#v found=%v", seen, found)
	}

	// The session is in the cookies, not in this process: another process
	// holding the same keyring resolves the same request.
	other := cookieManager(t, testKeyring(t, 1), c.Now)
	seen, found, _ = resolve(t, other, client)
	if !found || seen.AccountID != "account-1" {
		t.Fatalf("second process session = %#v found=%v", seen, found)
	}
}

func TestCookieStoreDefersRecordDecodeUntilSessionAccess(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := cookieManager(t, testKeyring(t, 1), c.Now)
	client := login(t, manager, newBrowser(), "account-1")

	tampered := client.copy()
	record := *tampered.cookies[DefaultDataCookieName]
	record.Value = record.Value[:len(record.Value)-1] + "A"
	tampered.cookies[DefaultDataCookieName] = &record

	untouched := run(manager, tampered.list(), func(http.ResponseWriter, *http.Request) {})
	if cleared := sessionCookie(t, untouched, DefaultCookieName); cleared != nil {
		t.Fatalf("a route with no session access decoded or cleared the record: %#v", cleared)
	}

	observed := run(manager, tampered.list(), func(_ http.ResponseWriter, r *http.Request) {
		_, _ = Load[payload](r.Context())
	})
	if cleared := sessionCookie(t, observed, DefaultCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("first session access did not reject the tampered record: %#v", cleared)
	}
}

func TestCookieStoreRejectsARecordFromAnotherSession(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := cookieManager(t, testKeyring(t, 1), c.Now)

	first := login(t, manager, newBrowser(), "account-1")
	second := login(t, manager, newBrowser(), "account-2")

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
	manager := cookieManager(t, testKeyring(t, 1), c.Now)
	client := login(t, manager, newBrowser(), "account-1")

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
	foreign := cookieManager(t, testKeyring(t, 9), c.Now)
	if _, found, _ := resolve(t, foreign, client); found {
		t.Fatal("a record sealed under another keyring was accepted")
	}
}

func TestCookieStoreRenewsAndExpiresLikeABackend(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := cookieManager(t, testKeyring(t, 1), c.Now)
	client := login(t, manager, newBrowser(), "account-1")
	issued := client.cookies[DefaultDataCookieName].Value

	// Before the renewal interval the record cookie is left alone.
	c.advance(time.Minute)
	if _, _, recorder := resolve(t, manager, client); sessionCookie(t, recorder, DefaultDataCookieName) != nil {
		t.Fatal("record cookie rewritten before the renewal interval")
	}
	// After it, idle expiry moves and the browser gets the new record.
	c.advance(10 * time.Minute)
	if _, found, _ := resolve(t, manager, client); !found {
		t.Fatal("session lost during renewal")
	}
	if client.cookies[DefaultDataCookieName].Value == issued {
		t.Fatal("record cookie was not renewed")
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
	manager := cookieManager(t, testKeyring(t, 1), c.Now)
	client := login(t, manager, newBrowser(), "account-1")
	before := *client.cookies[DefaultCookieName]

	login(t, manager, client, "account-1")
	if client.cookies[DefaultCookieName].Value == before.Value {
		t.Fatal("rotation reused the token")
	}
	seen, found, _ := resolve(t, manager, client)
	if !found || seen.AccountID != "account-1" {
		t.Fatalf("session after rotation = %#v found=%v", seen, found)
	}
}

func TestCookieStoreDestroyClearsTheBrowserButCannotRevoke(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := cookieManager(t, testKeyring(t, 1), c.Now)
	client := login(t, manager, newBrowser(), "account-1")
	kept := client.copy()

	recorder := run(manager, client.list(), func(w http.ResponseWriter, r *http.Request) {
		if err := manager.Destroy(w, r); err != nil {
			t.Fatalf("Destroy: %v", err)
		}
	})
	client.accept(recorder)
	if len(client.cookies) != 0 {
		t.Fatalf("cookies after Destroy = %#v", client.cookies)
	}
	if _, found, _ := resolve(t, manager, client); found {
		t.Fatal("the browser that logged out still has a session")
	}

	// What a browser holds, a browser keeps: this store cannot revoke a record
	// it already wrote, which is the reason a deployment that must end sessions
	// on demand uses a server-side store.
	if _, found, _ := resolve(t, manager, kept); !found {
		t.Fatal("a cookie backend appeared to revoke a record it cannot reach")
	}
}
