package pw

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/session"
)

func testSessionConfig(backend string) SessionConfig {
	return SessionConfig{
		Enabled: true,
		Backend: backend,
		Cookie: SessionCookieConfig{
			Name: "pw_session", Path: "/", Secure: true, HTTPOnly: true, SameSite: "strict",
		},
		CookieStore: SessionCookieStoreConfig{Name: session.DefaultDataCookieName},
	}
}

func TestCookieBackendNeedsNoImport(t *testing.T) {
	// This test binary imports no storage plugin, so a cookie-backed session
	// working here is the property the built-in backend exists for.
	config := testSessionConfig(SessionBackendCookie)
	config.CookieStore.Secret = base64.StdEncoding.EncodeToString(make([]byte, 32))

	backend, err := OpenSessionBackend(t.Context(), config, SessionResources{})
	if err != nil {
		t.Fatalf("cookie backend: %v", err)
	}
	if backend.Store == nil {
		t.Fatal("cookie backend returned no store")
	}
	// It opened nothing and accumulates nothing, so it hands back neither
	// responsibility.
	if backend.Close != nil || backend.Prune != nil {
		t.Fatal("cookie backend claimed a close or a sweep")
	}
	if _, ok := backend.Store.(session.RequestBinder); !ok {
		t.Fatal("cookie backend cannot reach the request it stores into")
	}
}

func TestUnimportedBackendNamesTheImport(t *testing.T) {
	// Nothing here imports sessionstore/sqlite or sessionstore/redis, which
	// is exactly the mistake this message exists for.
	for backend, path := range map[string]string{
		SessionBackendRDB:   "sessionstore/sqlite",
		SessionBackendRedis: "sessionstore/redis",
	} {
		_, err := OpenSessionBackend(t.Context(), testSessionConfig(backend), SessionResources{})
		if err == nil {
			t.Fatalf("%s opened without its plugin", backend)
		}
		if !strings.Contains(err.Error(), "import _") || !strings.Contains(err.Error(), path) {
			t.Fatalf("%s error = %v", backend, err)
		}
	}

	// An unknown name has no import to suggest, so the error lists what is
	// registered instead.
	_, err := OpenSessionBackend(t.Context(), testSessionConfig("cassandra"), SessionResources{})
	if err == nil || !strings.Contains(err.Error(), SessionBackendCookie) {
		t.Fatalf("unknown backend error = %v", err)
	}
}

func TestCookieBackendRequiresAUsableSecret(t *testing.T) {
	config := testSessionConfig(SessionBackendCookie)
	_, err := OpenSessionBackend(t.Context(), config, SessionResources{})
	if err == nil || !strings.Contains(err.Error(), "session.cookie_store.secret") {
		t.Fatalf("missing secret error = %v", err)
	}

	config.CookieStore.Secret = base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err = OpenSessionBackend(t.Context(), config, SessionResources{})
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("short secret error = %v", err)
	}

	// A rejected secret is still a secret: it must not reach the message a
	// startup failure prints.
	config.CookieStore.Secret = "hunter2-not-base64!"
	_, err = OpenSessionBackend(t.Context(), config, SessionResources{})
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("malformed secret error = %v", err)
	}
}

func TestCookieBackendKeepsRotatedSecretsReadable(t *testing.T) {
	config := testSessionConfig(SessionBackendCookie)
	config.CookieStore.Secret = base64.StdEncoding.EncodeToString(secretOf(2))
	config.CookieStore.PreviousSecrets = []string{base64.StdEncoding.EncodeToString(secretOf(1))}
	if _, err := OpenSessionBackend(t.Context(), config, SessionResources{}); err != nil {
		t.Fatalf("rotation: %v", err)
	}
}

func TestSessionCookiePolicyIsSharedByBothHalves(t *testing.T) {
	policy, err := SessionCookiePolicy(testSessionConfig(SessionBackendCookie))
	if err != nil {
		t.Fatal(err)
	}
	if policy.Name != "pw_session" || !policy.Secure || !policy.HTTPOnly ||
		policy.SameSite != http.SameSiteStrictMode {
		t.Fatalf("policy = %#v", policy)
	}
	config := testSessionConfig(SessionBackendCookie)
	config.Cookie.SameSite = "sideways"
	if _, err := SessionCookiePolicy(config); err == nil {
		t.Fatal("an unknown same-site value was accepted")
	}
}

func TestRegisterSessionBackendRejectsADuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a duplicate registration was accepted")
		}
	}()
	RegisterSessionBackend(SessionBackendCookie, func(context.Context, SessionConfig, SessionResources) (session.Backend, error) {
		return session.Backend{}, errors.New("unreachable")
	})
}

func secretOf(fill byte) []byte {
	secret := make([]byte, 32)
	for index := range secret {
		secret[index] = fill
	}
	return secret
}
