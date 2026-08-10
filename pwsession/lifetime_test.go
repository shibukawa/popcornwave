package pwsession

import (
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/sessionconfig"
)

// With no lifetime source the session is bounded by the browser: the token
// cookie carries no Max-Age and no absolute deadline is stamped.
func TestNoLifetimeSourceLeavesTheSessionToTheBrowser(t *testing.T) {
	options, err := Options(sessionconfig.SessionConfig{
		Enabled: true,
		Backend: sessionconfig.SessionBackendCookie,
		Cookie:  sessionconfig.SessionCookieConfig{Name: "pw_session", Path: "/", SameSite: "lax"},
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
	config := sessionconfig.SessionConfig{
		Enabled:   true,
		Backend:   sessionconfig.SessionBackendRDB,
		Retention: 0,
		Cookie:    sessionconfig.SessionCookieConfig{Name: "pw_session", Path: "/", SameSite: "lax"},
	}
	options, err := Options(config, sessionconfig.SessionLifetimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if options.TTL > 0 {
		t.Fatalf("TTL = %v, want none so the refusal has something to catch", options.TTL)
	}
	// The cookie backend keeps nothing on a server, so it is not refused.
	config.Backend = sessionconfig.SessionBackendCookie
	if _, err := Options(config, sessionconfig.SessionLifetimeConfig{}); err != nil {
		t.Fatalf("the cookie backend was refused an unbounded record: %v", err)
	}
}
