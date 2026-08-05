package jwt

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// keySetServer serves one JWKS and can be told to start failing, which is what
// an issuer becoming unreachable looks like from here.
type keySetServer struct {
	*httptest.Server
	failing atomic.Bool
	served  atomic.Int64
}

func newKeySetServer(t *testing.T) *keySetServer {
	t.Helper()
	secret := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	server := &keySetServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if server.failing.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		server.served.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kty":"oct","kid":"one","use":"sig","alg":"HS256","k":"` + secret + `"}]}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// staleKeySet returns a key set whose clock the test drives.
func staleKeySet(t *testing.T, server *keySetServer, now *time.Time, maxStale time.Duration) *RemoteKeySet {
	t.Helper()
	set, err := NewRemoteKeySet(server.URL, KeySourceOptions{
		Mode:              KeySetDirect,
		KeySetURI:         server.URL + "/jwks.json",
		AllowLoopbackHTTP: true,
		CacheTTL:          15 * time.Minute,
		MaxStaleAge:       maxStale,
		Clock:             func() time.Time { return *now },
	})
	if err != nil {
		t.Fatalf("NewRemoteKeySet: %v", err)
	}
	return set
}

func TestCachedKeysSurviveAnOutageAndThenStop(t *testing.T) {
	server := newKeySetServer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	set := staleKeySet(t, server, &now, time.Hour)
	ctx := context.Background()
	header := Header{Algorithm: "HS256", KeyID: "one"}

	if _, err := set.Resolve(ctx, header); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	server.failing.Store(true)

	// Within the cache TTL nothing is even refetched.
	now = now.Add(10 * time.Minute)
	if _, err := set.Resolve(ctx, header); err != nil {
		t.Fatalf("resolve inside the cache TTL: %v", err)
	}

	// Past the TTL the refresh fails, and the keys are still used: they did not
	// become untrustworthy because the issuer became unreachable.
	now = now.Add(30 * time.Minute)
	if _, err := set.Resolve(ctx, header); err != nil {
		t.Fatalf("resolve inside the stale window: %v", err)
	}

	// Past the stale window it stops. A key withdrawn from the published
	// document is withdrawn whether or not this process can read the document.
	now = now.Add(time.Hour)
	if _, err := set.Resolve(ctx, header); err == nil {
		t.Fatal("keys were still served past the stale window")
	}

	// Recovery restores service without a restart.
	server.failing.Store(false)
	if _, err := set.Resolve(ctx, header); err != nil {
		t.Fatalf("resolve after the issuer recovered: %v", err)
	}
}

func TestMaxStaleAgeDefaultsAndIsBounded(t *testing.T) {
	server := newKeySetServer(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	set := staleKeySet(t, server, &now, 0)
	if set.options.MaxStaleAge != defaultMaxStaleAge {
		t.Fatalf("MaxStaleAge default = %v, want %v", set.options.MaxStaleAge, defaultMaxStaleAge)
	}

	// There is deliberately no unbounded setting, so a value past the cap is
	// refused rather than quietly clamped.
	for _, value := range []time.Duration{-time.Second, maxKeySourceCacheTTL + time.Second} {
		_, err := NewRemoteKeySet(server.URL, KeySourceOptions{
			Mode:              KeySetDirect,
			KeySetURI:         server.URL + "/jwks.json",
			AllowLoopbackHTTP: true,
			MaxStaleAge:       value,
		})
		if !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("NewRemoteKeySet with MaxStaleAge %v = %v, want ErrInvalidOptions", value, err)
		}
	}
}

func TestUnknownKeyRefreshIsRateLimited(t *testing.T) {
	server := newKeySetServer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	set := staleKeySet(t, server, &now, time.Hour)
	ctx := context.Background()

	if _, err := set.Resolve(ctx, Header{Algorithm: "HS256", KeyID: "one"}); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	before := server.served.Load()

	// A stream of forged kid values must not be amplified into traffic against
	// the issuer: one refetch is allowed, and the cooldown covers the rest.
	for range 20 {
		if _, err := set.Resolve(ctx, Header{Algorithm: "HS256", KeyID: "forged"}); !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("resolve of an unknown kid = %v, want ErrKeyNotFound", err)
		}
	}
	if fetched := server.served.Load() - before; fetched > 1 {
		t.Fatalf("%d refetches for unknown kids, want at most 1", fetched)
	}
}
