package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// authenticateBearer is the ModeJWTOnly counterpart of authenticate.
//
// It mounts no endpoint and establishes no session: a bearer request carries its
// own credential, so there is nothing to correlate across requests and nothing
// to set a cookie for. Every request is authenticated from scratch, which is
// also why revocation exists — there is no session to destroy.
func (rt *runtime) authenticateBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, relaxed, err := rt.resolveBearer(r)
		switch {
		case errors.Is(err, ErrNoCredential):
			// An anonymous request is not a failure. The guard decides whether
			// this path needed a credential; an unprotected one serves without.
			next.ServeHTTP(w, r)
			return
		case err != nil:
			rt.refuseBearer(w, r, err)
			return
		}
		if relaxed {
			markDevResponse(w)
		}
		r = r.WithContext(pwruntime.WithAuthentication(r.Context(), pwruntime.Authentication{
			Authenticated:   true,
			Subject:         identity.AccountID,
			Method:          MethodBearer,
			Principal:       identity,
			AuthenticatedAt: identity.IssuedAt,
			ExpiresAt:       identity.ExpiresAt,
		}))
		next.ServeHTTP(w, r)
	})
}

// BearerIdentity is what a handler reads about the caller behind a bearer
// request. It is the verified identity plus the account admission resolved.
//
// It carries no token body: policy:access-token-verification excludes it, and a
// handler that needs to call another service must obtain its own credential
// rather than replay this one.
type BearerIdentity struct {
	// AccountID is the local account the identity resolved to.
	AccountID string
	// Account is the resolver's summary of that account.
	Account Account
	// Identity is the verified external identity, including its claims.
	Identity Identity
	// IssuedAt and ExpiresAt are the verified iat and exp.
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// resolveBearer verifies, admits, and checks revocation, in that order.
//
// The order is the point: nothing reaches an allowlist lookup or a revocation
// store until the signature has proved the issuer minted the token, so an
// unauthenticated caller cannot turn either into a query it controls.
func (rt *runtime) resolveBearer(r *http.Request) (BearerIdentity, bool, error) {
	verifier := rt.bearer
	if verifier == nil {
		return BearerIdentity{}, false, ErrInvalidToken
	}
	ctx := r.Context()

	identity, relaxed := verifier.devAdmits(r)
	if !relaxed {
		compact, err := verifier.bearerCredential(r)
		if err != nil {
			return BearerIdentity{}, false, err
		}
		if identity, err = verifier.verify(ctx, compact); err != nil {
			return BearerIdentity{}, false, err
		}
	}

	account, err := admitIdentity(ctx, rt.admissionFor(), rt.allowlist, identity)
	if err != nil {
		return BearerIdentity{}, false, err
	}
	if err := rt.revocations.check(ctx, identity); err != nil {
		if errors.Is(err, ErrRevokedToken) {
			return BearerIdentity{}, false, err
		}
		// The store could not answer. That is an unknown rather than a "not
		// revoked", so the deployment's fail-closed setting decides.
		pw.Logger(ctx).Log(ctx, pw.LevelError, "revocation lookup failed", pw.Err(err))
		if refusal := rt.revocations.onUnavailable(); refusal != nil {
			return BearerIdentity{}, false, refusal
		}
	}
	return BearerIdentity{
		AccountID: account.ID,
		Account:   account,
		Identity:  identity,
		IssuedAt:  identity.IssuedAt,
		ExpiresAt: identity.ExpiresAt,
	}, relaxed, nil
}

// refuseBearer answers one enumeration-safe 401.
//
// Every rejection returns the same status and the same body. A caller learns
// that the credential was not accepted and never which check rejected it,
// because the difference between "wrong audience", "expired", and "revoked" is
// an oracle for probing what this deployment trusts.
func (rt *runtime) refuseBearer(w http.ResponseWriter, r *http.Request, err error) {
	pw.Logger(r.Context()).Log(r.Context(), pw.LevelInfo, "bearer token refused", pw.Err(err))
	w.Header().Set("Cache-Control", "no-store")
	// RFC 6750 asks a protected resource to name the scheme it accepts. The
	// realm is the audience, which the caller already has to know to have asked
	// for a token, so naming it discloses nothing.
	w.Header().Set("WWW-Authenticate", `Bearer realm="`+rt.bearerRealm()+`", error="invalid_token"`)
	pw.WriteProblem(w, r, pw.Unauthorized())
}

func (rt *runtime) bearerRealm() string {
	if len(rt.config.JWT.Audience) == 0 {
		return "api"
	}
	// Quoted-string escaping: a realm is a quoted token, and an audience is
	// configuration rather than a constant.
	return quoteHeaderValue(rt.config.JWT.Audience[0])
}

func quoteHeaderValue(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r == '"' || r == '\\' || r < 0x20 || r == 0x7f {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
