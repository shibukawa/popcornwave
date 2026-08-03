package session

import (
	"net/http"
	"testing"
	"time"
)

// A deployment that declared no lifetime is bounded by the browser alone. The
// codec has always read a zero expiry stamp as "no expiry"; the record layer
// has to agree, or a session with no declared bound reads as already expired.
func TestASessionWithNoDeclaredLifetimeSurvives(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	store := newMapStore()
	options := Options{
		Cookie: CookieOptions{Secure: true, HTTPOnly: true},
		Keys:   testKeyring(t, 1),
		Now:    c.Now,
	}
	manager := testManager(t, accountRegistry(t), store, options)

	first := run(manager, nil, func(_ http.ResponseWriter, r *http.Request) {
		handle, _ := Value[payload](r.Context())
		if err := handle.Set(payload{AccountID: "no-ttl"}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	})
	if cookieOf(first, DefaultDataCookieName) == nil {
		t.Fatal("no record cookie was written")
	}
	run(manager, carry(first), func(_ http.ResponseWriter, r *http.Request) {
		value, ok := Load[payload](r.Context())
		if !ok {
			t.Fatal("the value did not survive one round trip with no TTL")
		}
		if value.AccountID != "no-ttl" {
			t.Fatalf("value = %#v", value)
		}
	})
}
