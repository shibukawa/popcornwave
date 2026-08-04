package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/jwt"
)

// accessTokenType is the media type RFC 9068 gives a JWT access token. Demanding
// it is what keeps an ID Token from being replayed here: both are signed by the
// same issuer with the same key, and this header is the field that says which
// one the issuer meant to mint.
const accessTokenType = "at+jwt"

// MethodBearer labels a request authenticated by a bearer access token. It never
// labels a session, because this mode creates none.
const MethodBearer = "bearer"

// errBearer categories. They are deliberately coarse: a caller learns that the
// credential was not accepted, never which check rejected it, because the
// difference between "wrong audience" and "expired" is a probing oracle.
var (
	// ErrNoCredential reports a request that carried no bearer token. It is not
	// a failure on its own: an unprotected path serves it anonymously.
	ErrNoCredential = errors.New("auth: no bearer credential")
	// ErrInvalidToken reports a credential that did not verify.
	ErrInvalidToken = errors.New("auth: invalid bearer token")
	// ErrRevokedToken reports a verified token that was withdrawn.
	ErrRevokedToken = errors.New("auth: revoked bearer token")
)

// bearerVerifier turns an Authorization header into a verified identity. It
// holds the key set and the parsed configuration, and is rebuilt whenever
// framework initialization runs.
type bearerVerifier struct {
	config JWTConfig
	keys   *jwt.RemoteKeySet
	// now is injectable so tests can move the clock without sleeping.
	now func() time.Time
}

func newBearerVerifier(config JWTConfig) (*bearerVerifier, error) {
	mode := jwt.DiscoverOpenID
	switch config.Discovery {
	case DiscoveryOAuth:
		mode = jwt.DiscoverAuthorizationServer
	case DiscoveryManual:
		mode = jwt.KeySetDirect
	}
	keys, err := jwt.NewRemoteKeySet(config.Issuer, jwt.KeySourceOptions{
		Mode:              mode,
		KeySetURI:         config.JWKSURI,
		AllowLoopbackHTTP: config.AllowLoopbackHTTP,
		RefreshCooldown:   config.JWKSRefreshCooldown,
	})
	if err != nil {
		return nil, fmt.Errorf("auth.jwt: %w", err)
	}
	return &bearerVerifier{config: config, keys: keys, now: time.Now}, nil
}

// bearerCredential extracts the compact token from the Authorization header.
//
// The scheme is matched case-insensitively, as RFC 7235 requires, and the token
// is bounded before anything decodes it. A second Authorization header is
// refused rather than merged: which one a proxy forwards is not this
// application's decision to guess.
func (v *bearerVerifier) bearerCredential(r *http.Request) (string, error) {
	values := r.Header.Values("Authorization")
	switch len(values) {
	case 0:
		return "", ErrNoCredential
	case 1:
	default:
		return "", ErrInvalidToken
	}
	scheme, token, found := strings.Cut(values[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", ErrNoCredential
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrInvalidToken
	}
	if len(token) > v.config.MaxTokenBytes {
		return "", ErrInvalidToken
	}
	return token, nil
}

// verify applies policy:access-token-verification to a compact token and
// returns the verified identity.
//
// The order matters: signature and algorithm first, then the claims, then the
// required set. Nothing reaches an allowlist lookup or a revocation store until
// the signature has proved the issuer minted it.
func (v *bearerVerifier) verify(ctx context.Context, compact string) (Identity, error) {
	token, err := jwt.Parse(compact, jwt.ParseOptions{MaxTokenBytes: v.config.MaxTokenBytes})
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	if err := v.checkTokenType(token.Header); err != nil {
		return Identity{}, err
	}
	claims, err := jwt.Verify(token, v.keys.ResolverFor(ctx), jwt.VerifyOptions{
		AllowedAlgorithms: v.config.Algorithms,
		Issuer:            v.config.Issuer,
		Leeway:            v.config.Leeway,
		Clock:             v.now,
		// The audience and the token type are compared here rather than by the
		// verifier: it holds one audience string and one exact typ, and this
		// mode has a configured list and a media type with two spellings.
	})
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	if err := v.checkRequiredClaims(claims); err != nil {
		return Identity{}, err
	}
	if err := v.checkAudience(claims); err != nil {
		return Identity{}, err
	}
	if err := v.checkLifetime(claims); err != nil {
		return Identity{}, err
	}
	if err := v.checkScopes(claims); err != nil {
		return Identity{}, err
	}
	identity := identityFrom(claims, v.config.IdentityClaim)
	if identity.Key == "" {
		// No usable value for the configured identity claim means no account
		// can be named, so nothing downstream may treat this as a known caller.
		return Identity{}, ErrInvalidToken
	}
	identity.TokenID = claims.ID
	if claims.IssuedAt != nil {
		identity.IssuedAt = time.Unix(*claims.IssuedAt, 0).UTC()
	}
	if claims.ExpiresAt != nil {
		identity.ExpiresAt = time.Unix(*claims.ExpiresAt, 0).UTC()
	}
	return identity, nil
}

// checkTokenType compares the typ header against the configured media type.
//
// RFC 9068 permits both "at+jwt" and the fully qualified
// "application/at+jwt", and media types are case-insensitive, so the comparison
// normalizes both rather than demanding one spelling.
func (v *bearerVerifier) checkTokenType(header jwt.Header) error {
	want := v.config.RequiredTokenType
	if want == "" {
		// An issuer predating RFC 9068 mints no typ. The deployment accepted
		// that cost explicitly and compensates with an audience the issuer does
		// not put in its ID Tokens.
		return nil
	}
	if normalizeMediaType(header.Type) != normalizeMediaType(want) {
		return ErrInvalidToken
	}
	return nil
}

func normalizeMediaType(value string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "application/")
}

// checkRequiredClaims enforces the RFC 9068 required set.
//
// contrib/jwt requires exp and validates the rest when present. An optional
// check is a check some deployment will find switched off after an incident, so
// the whole set is demanded here: iss and exp are already proved by the
// verifier, and sub, iat, and jti are proved now.
func (v *bearerVerifier) checkRequiredClaims(claims jwt.Claims) error {
	if claims.Subject == "" {
		return ErrInvalidToken
	}
	if claims.IssuedAt == nil {
		return ErrInvalidToken
	}
	if len(claims.Audience) == 0 {
		return ErrInvalidToken
	}
	if v.config.requiresJTI() && claims.ID == "" {
		return ErrInvalidToken
	}
	return nil
}

// checkAudience compares the token's aud against the configured list.
//
// any is the ordinary answer: an access token names every resource it may
// reach, so this API appearing among them is the question. all exists for a
// deployment that mints a token per resource pair and wants both named.
func (v *bearerVerifier) checkAudience(claims jwt.Claims) error {
	if v.config.AudienceMatch == MatchAll {
		for _, want := range v.config.Audience {
			if !containsString(claims.Audience, want) {
				return ErrInvalidToken
			}
		}
		return nil
	}
	for _, want := range v.config.Audience {
		if containsString(claims.Audience, want) {
			return nil
		}
	}
	return ErrInvalidToken
}

// checkLifetime refuses a token that outlives what the deployment declared.
//
// A token whose exp minus iat exceeds the configured maximum would also outlive
// a subject-form revocation entry, which is retained for exactly that long:
// accepting it would leave a revoked identity working again once the entry aged
// out.
func (v *bearerVerifier) checkLifetime(claims jwt.Claims) error {
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return ErrInvalidToken
	}
	lifetime := time.Duration(*claims.ExpiresAt-*claims.IssuedAt) * time.Second
	if lifetime <= 0 || lifetime > v.config.MaxTokenLifetime {
		return ErrInvalidToken
	}
	return nil
}

// checkScopes requires every configured scope value.
//
// scope is a space-delimited string rather than an array, so a generic claim
// comparison would match the whole value and admit "admin" for a token holding
// "not-admin". Splitting it is the reason this is its own field.
func (v *bearerVerifier) checkScopes(claims jwt.Claims) error {
	if len(v.config.RequiredScopes) == 0 {
		return nil
	}
	raw, ok := claims.Raw["scope"]
	if !ok {
		return ErrInvalidToken
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ErrInvalidToken
	}
	granted := strings.Fields(value)
	for _, want := range v.config.RequiredScopes {
		if !containsString(granted, want) {
			return ErrInvalidToken
		}
	}
	return nil
}
