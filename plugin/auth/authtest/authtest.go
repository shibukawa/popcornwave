// Package authtest installs an authenticated request context without a server,
// a database, or a ceremony.
//
// It exists because most application tests treat authentication as a
// precondition rather than as the subject: they need to say which account a
// request belongs to and then test what the handler does. Running a WebAuthn
// ceremony to reach that point proves nothing about the handler and costs a
// port, a database, and an authenticator.
//
// A test may call a bare handler or ServeHTTP against the real middleware
// chain; the installed value survives both, because the session middleware
// leaves the context untouched on its unauthenticated path. The same test reads
// identically under every auth.mode, since this is exactly the value the resolve
// middleware would have installed.
//
// # Why this is not a bypass
//
// The carrier is a context value, which no remote client can set: net/http
// creates a fresh context for an incoming request, so nothing that arrives over
// a connection can reach it. A header would be the opposite, which is why
// plugin/auth reads none.
//
// The package must never appear in an application binary. It cannot forge an
// identity anywhere it is absent, and pw build rejects it in a dependency graph.
package authtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

// Identity is what a test says the request is. Only AccountID is required.
type Identity struct {
	// AccountID is the stable application identifier. It also becomes the
	// authentication subject, matching what plugin/auth records.
	AccountID   string
	DisplayName string
	Email       string
	// Issuer, Subject, KeyClaim, and Key describe an external identity. They
	// are what an OIDC login records and are empty for a passkey session.
	Issuer   string
	Subject  string
	KeyClaim string
	Key      string
	// Method defaults to auth.MethodOIDC. Use auth.MethodPasskey to model an
	// account that logged in with a credential.
	Method string
	// AuthenticatedAt defaults to now, so a handler gated on recent
	// authentication passes unless the test wants it not to.
	AuthenticatedAt time.Time
	// ExpiresAt defaults to an hour out.
	ExpiresAt time.Time
	// Scope carries optional tenant, role, or permission values for an
	// application authorization check.
	Scope []string
}

func (i Identity) normalized() Identity {
	if i.Method == "" {
		i.Method = auth.MethodOIDC
	}
	if i.AuthenticatedAt.IsZero() {
		i.AuthenticatedAt = time.Now()
	}
	if i.ExpiresAt.IsZero() {
		i.ExpiresAt = i.AuthenticatedAt.Add(time.Hour)
	}
	return i
}

func (i Identity) sessionData() auth.SessionData {
	return auth.SessionData{
		AccountID:       i.AccountID,
		AuthenticatedAt: i.AuthenticatedAt,
		Method:          i.Method,
		Issuer:          i.Issuer,
		Subject:         i.Subject,
		KeyClaim:        i.KeyClaim,
		Key:             i.Key,
		DisplayName:     i.DisplayName,
		Email:           i.Email,
	}
}

// NewContext returns ctx carrying an authenticated request, exactly as the
// session middleware would have installed it. auth.User, auth.Session,
// pw.Authenticated, and pw.RequestAuthentication all read it.
func NewContext(ctx context.Context, identity Identity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	identity = identity.normalized()
	data := identity.sessionData()
	// The login half of a session is one registered slot, so installing it is
	// the whole of what the middleware would have resolved.
	ctx = session.WithValue(ctx, data)
	return pwruntime.WithAuthentication(ctx, pwruntime.Authentication{
		Authenticated: true,
		// plugin/auth reports the account identifier as the subject, so a test
		// and a real login agree on what pw.RequestAuthentication().Subject is.
		Subject:         identity.AccountID,
		Method:          identity.Method,
		Principal:       data,
		Scope:           identity.Scope,
		AuthenticatedAt: identity.AuthenticatedAt,
		ExpiresAt:       identity.ExpiresAt,
	})
}

// Anonymous returns ctx in the explicitly unauthenticated state, so a test can
// prove the deny path rather than relying on a value simply being absent.
func Anonymous(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = session.WithValue(ctx, auth.SessionData{})
	return pwruntime.WithAuthentication(ctx, pwruntime.Authentication{})
}

// NewRequest returns an httptest request that already carries the identity. It
// is the usual entry point: build the request, hand it to a handler, assert on
// what the handler did.
func NewRequest(method, target string, body io.Reader, identity Identity) *http.Request {
	request := httptest.NewRequest(method, target, body)
	return request.WithContext(NewContext(request.Context(), identity))
}

// NewAnonymousRequest returns an httptest request in the explicitly
// unauthenticated state.
func NewAnonymousRequest(method, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	return request.WithContext(Anonymous(request.Context()))
}
