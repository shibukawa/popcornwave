package authtest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/plugin/auth/authtest"
	"github.com/shibukawa/popcornwave/pw"
)

// handler reports what the framework accessors see, which is what an
// application handler would branch on.
func handler(w http.ResponseWriter, r *http.Request) {
	authentication := pw.RequestAuthentication(r)
	if !authentication.Authenticated {
		_, _ = w.Write([]byte("anonymous"))
		return
	}
	user, ok := auth.User(r.Context())
	if !ok {
		_, _ = w.Write([]byte("authenticated but no session"))
		return
	}
	_, _ = w.Write([]byte(authentication.Method + ":" + authentication.Subject + ":" + user.DisplayName))
}

func serve(t *testing.T, request *http.Request) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder.Body.String()
}

func TestEveryAccessorReadsTheInstalledIdentity(t *testing.T) {
	body := serve(t, authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{
		AccountID: "account-1", DisplayName: "Alice",
	}))
	if body != "oidc:account-1:Alice" {
		t.Fatalf("body = %q, want oidc:account-1:Alice", body)
	}
}

// The value is the same under every mode, so an application test does not
// change when a deployment switches from OIDC to passkeys.
func TestMethodIsSelectable(t *testing.T) {
	body := serve(t, authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{
		AccountID: "account-1", DisplayName: "Alice", Method: auth.MethodPasskey,
	}))
	if body != "passkey:account-1:Alice" {
		t.Fatalf("body = %q, want a passkey session", body)
	}
}

func TestAnonymousIsExplicit(t *testing.T) {
	if body := serve(t, authtest.NewAnonymousRequest(http.MethodGet, "/", nil)); body != "anonymous" {
		t.Fatalf("body = %q, want anonymous", body)
	}
	// A plain request carries nothing either, which is the same deny path.
	if body := serve(t, httptest.NewRequest(http.MethodGet, "/", nil)); body != "anonymous" {
		t.Fatalf("plain request body = %q, want anonymous", body)
	}
}

// A handler gated on recent authentication needs to be able to fail, so the
// timestamp is settable.
func TestAuthenticatedAtIsSettableAndDefaultsToNow(t *testing.T) {
	fresh := authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{AccountID: "account-1"})
	view, ok := auth.Session(fresh.Context())
	if !ok {
		t.Fatal("no session installed")
	}
	if time.Since(view.AuthenticatedAt) > time.Minute {
		t.Fatalf("AuthenticatedAt = %v, want roughly now", view.AuthenticatedAt)
	}

	stale := authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{
		AccountID:       "account-1",
		AuthenticatedAt: time.Now().Add(-2 * time.Hour),
	})
	staleView, _ := auth.Session(stale.Context())
	if time.Since(staleView.AuthenticatedAt) < time.Hour {
		t.Fatalf("AuthenticatedAt = %v, want the value the test set", staleView.AuthenticatedAt)
	}
}

func TestExternalIdentityFieldsSurvive(t *testing.T) {
	request := authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{
		AccountID: "account-1", Issuer: "https://issuer.example", Subject: "sub-1",
		KeyClaim: "employee_number", Key: "E-42", Email: "alice@example.com",
	})
	user, ok := auth.User(request.Context())
	if !ok {
		t.Fatal("no session installed")
	}
	if user.Issuer != "https://issuer.example" || user.Subject != "sub-1" ||
		user.KeyClaim != "employee_number" || user.Key != "E-42" || user.Email != "alice@example.com" {
		t.Fatalf("session data = %+v, want the external identity the test set", user)
	}
}

func TestScopeReachesTheAuthorizationCheck(t *testing.T) {
	request := authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{
		AccountID: "account-1", Scope: []string{"admin", "billing"},
	})
	scope := pw.RequestAuthentication(request).Scope
	if len(scope) != 2 || scope[0] != "admin" {
		t.Fatalf("scope = %v, want the values the test set", scope)
	}
}

// The installed value survives a middleware chain, which is what lets a test
// exercise the real guard without a server.
func TestTheIdentitySurvivesAMiddlewareChain(t *testing.T) {
	var seen string
	chain := passthrough(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = pw.RequestAuthentication(r).Subject
	}))
	chain.ServeHTTP(httptest.NewRecorder(),
		authtest.NewRequest(http.MethodGet, "/", nil, authtest.Identity{AccountID: "account-1"}))
	if seen != "account-1" {
		t.Fatalf("subject after the chain = %q, want account-1", seen)
	}
}

// passthrough stands in for a middleware that does not authenticate, which is
// how the session middleware behaves on its unauthenticated path.
func passthrough(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func TestNilContextIsUsable(t *testing.T) {
	//nolint:staticcheck // a nil context is what a careless caller passes.
	if ctx := authtest.NewContext(nil, authtest.Identity{AccountID: "account-1"}); !pw.AuthenticatedContext(ctx) {
		t.Fatal("a nil context produced an unauthenticated result")
	}
	//nolint:staticcheck
	if ctx := authtest.Anonymous(nil); pw.AuthenticatedContext(ctx) {
		t.Fatal("Anonymous produced an authenticated result")
	}
}
