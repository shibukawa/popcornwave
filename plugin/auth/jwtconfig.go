package auth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// maxJWTLeeway bounds the configured clock-skew allowance. A leeway wide enough
// to matter is a expiry check nobody is doing.
const maxJWTLeeway = 5 * time.Minute

// maxConfiguredTokenLifetime bounds what a deployment may declare its issuer
// mints. It is not a limit on the issuer, which this application does not
// control; it is a limit on how long a subject-form revocation entry has to be
// retained, and on how far the exp-minus-iat sanity check can be widened before
// it stops meaning anything.
const maxConfiguredTokenLifetime = 24 * time.Hour

// hardMaxTokenBytes bounds the compact token this deployment will decode at all,
// whatever the configuration asks for.
const hardMaxTokenBytes = 64 * 1024

// validateJWTUse validates the bearer settings in the mode that uses them and
// refuses them everywhere else, so a leftover AUTH_JWT_ISSUER cannot suggest
// that this deployment accepts bearer tokens when it does not.
func (c Config) validateJWTUse() error {
	if !c.usesJWT() {
		return c.refuseJWTSettings()
	}
	return c.JWT.validate(c)
}

// refuseJWTSettings reports a bearer setting present in a mode that never reads
// one. Only the fields with no meaningful zero value are checked, because a
// bound default is not evidence that anybody wrote it down.
func (c Config) refuseJWTSettings() error {
	if c.JWT.Issuer != "" {
		return fmt.Errorf("auth.mode %q verifies no bearer token, but auth.jwt.issuer is set", c.Mode)
	}
	if len(c.JWT.Audience) != 0 {
		return fmt.Errorf("auth.mode %q verifies no bearer token, but auth.jwt.audience is set", c.Mode)
	}
	if c.JWT.Admission != "" {
		return fmt.Errorf("auth.mode %q verifies no bearer token, but auth.jwt.admission is set", c.Mode)
	}
	if c.JWT.Revocation.Mode != "" {
		return fmt.Errorf("auth.mode %q verifies no bearer token, but auth.jwt.revocation.mode is set", c.Mode)
	}
	if c.JWT.Dev.TrustUnverifiedTokens {
		return fmt.Errorf("auth.mode %q verifies no bearer token, but auth.jwt.dev.trust_unverified_tokens is set", c.Mode)
	}
	return nil
}

// validate applies the rules of requirement:jwt-only-api-authentication. Every
// field with a permissive answer is required rather than defaulted, so a
// configuration that forgot one fails at startup naming the key instead of
// serving something nobody chose.
func (j JWTConfig) validate(parent Config) error {
	if err := j.validateIssuer(); err != nil {
		return err
	}
	// A token verified without an audience check was minted for some other
	// service, and nothing else in the token says it was not meant for us.
	if len(j.Audience) == 0 {
		return errors.New("auth.jwt.audience must name this API; a token verified without an audience was minted for some other service")
	}
	for _, audience := range j.Audience {
		if strings.TrimSpace(audience) == "" {
			return errors.New("auth.jwt.audience must not contain an empty value")
		}
	}
	if err := j.validateAlgorithms(); err != nil {
		return err
	}
	if err := j.validateDiscovery(); err != nil {
		return err
	}
	if err := j.validateBounds(); err != nil {
		return err
	}
	if err := j.validateAdmission(); err != nil {
		return err
	}
	if err := j.validateRevocation(); err != nil {
		return err
	}
	if !validClaimName(j.IdentityClaim) {
		return fmt.Errorf("auth.jwt.identity_claim %q is not a valid claim name", j.IdentityClaim)
	}
	for _, scope := range j.RequiredScopes {
		if scope == "" || strings.ContainsAny(scope, " \t") {
			return fmt.Errorf("auth.jwt.required_scopes entry %q must be a single non-empty scope value", scope)
		}
	}
	// An API answers 401; it has no login page to send a caller to, and a 303
	// to one would be answered by a client that cannot render it.
	if parent.Protection.Unauthenticated != UnauthenticatedUnauthorized {
		return fmt.Errorf("auth.mode %q requires auth.protection.unauthenticated = %q; there is no login path to redirect a bearer client to",
			ModeJWTOnly, UnauthenticatedUnauthorized)
	}
	return nil
}

// validateIssuer requires an https issuer, or a loopback http one under the
// development allowance. The issuer is where trust in a signing key starts, so
// a transport that can be rewritten in flight makes every later check
// decorative.
func (j JWTConfig) validateIssuer() error {
	if j.Issuer == "" {
		return errors.New("auth.jwt.issuer must be set")
	}
	parsed, err := url.Parse(j.Issuer)
	if err != nil {
		return fmt.Errorf("auth.jwt.issuer is not a URL: %w", err)
	}
	if parsed.Fragment != "" || parsed.RawQuery != "" {
		return errors.New("auth.jwt.issuer must carry no query or fragment")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		// Hostname strips the port, which isLoopbackHost does not parse.
		if j.AllowLoopbackHTTP && isLoopbackHost(parsed.Hostname()) {
			return nil
		}
		return errors.New("auth.jwt.issuer must be https, or a loopback http URL with auth.jwt.allow_loopback_http = true")
	default:
		return fmt.Errorf("auth.jwt.issuer must be an http or https URL, got scheme %q", parsed.Scheme)
	}
}

// validateAlgorithms refuses an empty allowlist and every symmetric algorithm.
//
// HMAC is refused rather than supported because the verification key here comes
// from a published JWKS: accepting HS256 would let anyone holding that public
// document use it as the shared secret and mint their own tokens.
func (j JWTConfig) validateAlgorithms() error {
	if len(j.Algorithms) == 0 {
		return errors.New("auth.jwt.algorithms must name at least one algorithm; the token header never selects it")
	}
	for _, algorithm := range j.Algorithms {
		switch algorithm {
		case "RS256", "RS384", "RS512":
		case "none", "":
			return errors.New(`auth.jwt.algorithms must not contain "none"`)
		default:
			if strings.HasPrefix(algorithm, "HS") {
				return fmt.Errorf("auth.jwt.algorithms must not contain %q: the verification key is a published JWKS entry, so a symmetric algorithm would let anyone holding it mint tokens", algorithm)
			}
			return fmt.Errorf("auth.jwt.algorithms contains unsupported algorithm %q", algorithm)
		}
	}
	return nil
}

func (j JWTConfig) validateDiscovery() error {
	switch j.Discovery {
	case DiscoveryOIDC, DiscoveryOAuth:
		if j.JWKSURI != "" {
			return fmt.Errorf("auth.jwt.jwks_uri is read only under discovery %q, but discovery is %q", DiscoveryManual, j.Discovery)
		}
		return nil
	case DiscoveryManual:
		if j.JWKSURI == "" {
			return fmt.Errorf("auth.jwt.jwks_uri must be set under discovery %q", DiscoveryManual)
		}
		return j.validateKeySetOrigin(j.JWKSURI)
	default:
		return fmt.Errorf("auth.jwt.discovery must be %q, %q, or %q", DiscoveryOIDC, DiscoveryOAuth, DiscoveryManual)
	}
}

// validateKeySetOrigin requires the key set to share the issuer's origin.
//
// A metadata document that may point the key source at another host makes the
// issuer check decorative: whoever controls that host controls which signatures
// verify. The loopback allowance relaxes it for development, where the issuer
// and the key set are the same local process anyway.
func (j JWTConfig) validateKeySetOrigin(raw string) error {
	keys, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("auth.jwt.jwks_uri is not a URL: %w", err)
	}
	issuer, err := url.Parse(j.Issuer)
	if err != nil {
		return fmt.Errorf("auth.jwt.issuer is not a URL: %w", err)
	}
	if j.AllowLoopbackHTTP && isLoopbackHost(keys.Hostname()) && isLoopbackHost(issuer.Hostname()) {
		return nil
	}
	if keys.Scheme != "https" {
		return errors.New("auth.jwt.jwks_uri must be https")
	}
	if !strings.EqualFold(keys.Host, issuer.Host) {
		return fmt.Errorf("auth.jwt.jwks_uri host %q must match the auth.jwt.issuer host %q; a key set on another origin makes the issuer check decorative",
			keys.Host, issuer.Host)
	}
	return nil
}

func (j JWTConfig) validateBounds() error {
	if j.Leeway < 0 {
		return errors.New("auth.jwt.leeway must not be negative")
	}
	if j.Leeway > maxJWTLeeway {
		return fmt.Errorf("auth.jwt.leeway must not exceed %s; a wider allowance is an expiry check nobody is doing", maxJWTLeeway)
	}
	if j.MaxTokenLifetime <= 0 {
		return errors.New("auth.jwt.max_token_lifetime must be positive; it bounds the exp-minus-iat check and the retention of a subject-form revocation entry, and this application cannot know how long the issuer mints for")
	}
	if j.MaxTokenLifetime > maxConfiguredTokenLifetime {
		return fmt.Errorf("auth.jwt.max_token_lifetime must not exceed %s", maxConfiguredTokenLifetime)
	}
	if j.MaxTokenBytes <= 0 {
		return errors.New("auth.jwt.max_token_bytes must be positive")
	}
	if j.MaxTokenBytes > hardMaxTokenBytes {
		return fmt.Errorf("auth.jwt.max_token_bytes must not exceed %d", hardMaxTokenBytes)
	}
	if j.JWKSRefreshCooldown < 0 {
		return errors.New("auth.jwt.jwks_refresh_cooldown must not be negative")
	}
	return nil
}

func (j JWTConfig) validateAdmission() error {
	if j.Admission == "" {
		return fmt.Errorf("auth.jwt.admission must be set to %q, %q, %q, or %q; there is no default, because the permissive answer would be the silent one",
			AdmissionAuthenticated, AdmissionClaim, AdmissionRegistered, AdmissionExisting)
	}
	if err := validateChoice("auth.jwt.admission", j.Admission,
		AdmissionAuthenticated, AdmissionClaim, AdmissionRegistered, AdmissionExisting); err != nil {
		return err
	}
	switch j.Admission {
	case AdmissionClaim:
		if j.Claim.Path == "" || len(j.Claim.Values) == 0 {
			return fmt.Errorf("auth.jwt.admission %q requires auth.jwt.claim.path and auth.jwt.claim.values", AdmissionClaim)
		}
		if !strings.HasPrefix(j.Claim.Path, "/") {
			return errors.New("auth.jwt.claim.path must be a JSON Pointer starting with /")
		}
		if err := validateChoice("auth.jwt.claim.match", j.Claim.Match, MatchAny, MatchAll); err != nil {
			return err
		}
	case AdmissionExisting:
		if j.AutoProvision {
			return fmt.Errorf("auth.jwt.admission %q forbids auth.jwt.auto_provision", AdmissionExisting)
		}
	}
	for _, claim := range j.RegisteredClaims {
		if !validClaimName(claim) {
			return fmt.Errorf("auth.jwt.registered_claims entry %q is not a valid claim name", claim)
		}
	}
	return nil
}

// validateRevocation requires the deployment to say whether it can withdraw a
// token, and refuses the settings a chosen mode cannot honor.
func (j JWTConfig) validateRevocation() error {
	if j.Revocation.Mode == "" {
		return fmt.Errorf("auth.jwt.revocation.mode must be set to %q, %q, %q, or %q; running without a revocation path is a decision that belongs in the configuration rather than in an omission",
			RevocationOff, RevocationToken, RevocationSubject, RevocationBoth)
	}
	if err := validateChoice("auth.jwt.revocation.mode", j.Revocation.Mode,
		RevocationOff, RevocationToken, RevocationSubject, RevocationBoth); err != nil {
		return err
	}
	if err := validateChoice("auth.jwt.revocation.on_unavailable", j.Revocation.OnUnavailable,
		RevocationRefuse, RevocationAdmit); err != nil {
		return err
	}
	if j.Revocation.MaxPropagationDelay < 0 {
		return errors.New("auth.jwt.revocation.max_propagation_delay must not be negative")
	}
	if !j.Revocation.enabled() {
		if j.Revocation.MaxPropagationDelay > 0 {
			return fmt.Errorf("auth.jwt.revocation.max_propagation_delay is read only when revocation is on, but the mode is %q", j.Revocation.Mode)
		}
		return nil
	}
	// A cache longer than the tokens it answers about would outlive every token
	// it could have refused, which is a cache that never expires a wrong answer.
	if j.Revocation.MaxPropagationDelay >= j.MaxTokenLifetime {
		return fmt.Errorf("auth.jwt.revocation.max_propagation_delay must be shorter than auth.jwt.max_token_lifetime (%s)", j.MaxTokenLifetime)
	}
	return nil
}

// requiresJTI reports whether a token must carry a jti to be accepted.
//
// RFC 9068 requires one of an at+jwt token, so the ordinary configuration
// demands it already. Selecting the token revocation form extends the demand to
// a deployment that relaxed the token type, because a token nobody can name is
// a token nobody can revoke.
func (j JWTConfig) requiresJTI() bool {
	return j.RequiredTokenType == accessTokenType || j.Revocation.revokesTokens()
}
