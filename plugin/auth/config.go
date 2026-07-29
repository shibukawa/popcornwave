package auth

import (
	"fmt"
	"strings"
)

// Authentication modes. Only ModeOIDCOnly is implemented; the passkey modes are
// rejected during startup validation until their flows exist.
const (
	ModeOIDCOnly    = "oidc_only"
	ModeOIDCPasskey = "oidc_passkey"
	ModePasskeyOnly = "passkey_only"
)

// Admission policies applied to a verified OIDC identity.
const (
	// AdmissionAuthenticated admits every identity the configured issuer
	// verifies.
	AdmissionAuthenticated = "authenticated"
	// AdmissionClaim admits an identity whose configured claim matches.
	AdmissionClaim = "claim"
	// AdmissionExisting admits only an identity an application resolver
	// already knows. It forbids provisioning.
	AdmissionExisting = "existing"
	// AdmissionRegistered admits only an identity a deployment registered in
	// advance in the framework-owned allowlist table. It is the mode for a
	// closed deployment whose users are known before their first login.
	AdmissionRegistered = "registered"
)

// Responses to an unauthenticated request on a protected path.
const (
	UnauthenticatedRedirect     = "redirect"
	UnauthenticatedUnauthorized = "unauthorized"
)

// Claim match modes.
const (
	MatchAny = "any"
	MatchAll = "all"
)

// Config is the [auth] runtime binding. It is registered when this package is
// imported.
type Config struct {
	Enabled bool   `default:"false"`
	Mode    string `default:"oidc_only" dependon:".enabled" help:"oidc_only"`
	// LoginPath starts the provider flow.
	LoginPath    string `default:"/auth/login" dependon:".enabled" help:"path that starts the provider flow"`
	CallbackPath string `default:"/auth/callback" dependon:".enabled"`
	LogoutPath   string `default:"/auth/logout" dependon:".enabled"`
	// PostLoginPath is the local path a completed login lands on.
	PostLoginPath string           `default:"/" dependon:".enabled" help:"path a completed login lands on"`
	Protection    ProtectionConfig `dependon:".enabled"`
	OIDC          OIDCConfig       `dependon:".enabled"`
}

// ProtectionConfig selects the paths that require an authenticated request.
type ProtectionConfig struct {
	Include []string `help:"protected path pattern"`
	Exclude []string `help:"public path pattern"`
	// Unauthenticated is redirect or unauthorized.
	Unauthenticated string `default:"redirect" help:"redirect or unauthorized"`
}

// OIDCConfig describes the relying-party registration and admission policy.
type OIDCConfig struct {
	Issuer       string `env:"AUTH_OIDC_ISSUER"`
	ClientID     string `env:"AUTH_OIDC_CLIENT_ID"`
	ClientSecret string `secret:"mask" env:"AUTH_OIDC_CLIENT_SECRET"`
	RedirectURL  string
	Scopes       []string
	// IdentityClaim names the verified claim that identifies a local account.
	// It defaults to "sub".
	//
	// A deployment that provisions users in advance usually cannot know a
	// subject yet, and instead adds its own stable identifier to the
	// directory, such as an employee number. That claim must be stable for the
	// life of the account and unique within the issuer: it becomes the account
	// link, so a reissued or reused value hands one person another person's
	// account.
	IdentityClaim string `default:"sub" help:"verified claim that identifies a local account"`
	Admission     string `default:"authenticated" help:"authenticated, claim, registered, or existing"`
	// AutoProvision permits an unknown verified identity to create an account
	// through the registered account resolver.
	AutoProvision bool `default:"true" help:"AutoProvision permits an unknown verified identity to create an account through the registered account resolver"`
	// Claim is the admission rule applied when Admission is claim.
	Claim ClaimConfig `help:"admission rule applied when admission is claim"`
	// RegisteredClaims names the verified claims compared against the
	// allowlist table under AdmissionRegistered. It defaults to IdentityClaim
	// alone, because that is the value a deployment registers in advance.
	RegisteredClaims []string `help:"claims compared against the allowlist; defaults to identity_claim"`
	// ProviderLogout ends the provider session as well, through the
	// discovered end session endpoint. Without it the provider stays signed
	// in, so the next login returns the same user without asking and the
	// sign-out looks like it did nothing.
	ProviderLogout bool `default:"true" help:"also end the provider session on logout"`
	// AllowLoopbackHTTP permits an http issuer on localhost. It exists for
	// local development against a loopback identity provider and must stay
	// false everywhere else.
	AllowLoopbackHTTP bool `default:"false" help:"permit an http loopback issuer during development"`
}

// ClaimConfig is the admission rule of AdmissionClaim. Its keys hang off
// auth.enabled rather than the admission policy: falsy names the single value
// that means off, and here every policy except claim would have to be named.
type ClaimConfig struct {
	// Path is a JSON Pointer into the verified ID Token claims.
	Path   string `help:"JSON Pointer into verified claims"`
	Values []string
	Match  string `default:"any" help:"any or all"`
}

// validate rejects a configuration this build cannot serve. It runs during
// framework initialization so a deployment fails before accepting requests.
func (c Config) validate() error {
	switch c.Mode {
	case ModeOIDCOnly:
	case ModeOIDCPasskey, ModePasskeyOnly:
		return fmt.Errorf("auth.mode %q is not implemented yet; use %q", c.Mode, ModeOIDCOnly)
	default:
		return fmt.Errorf("auth.mode must be %q", ModeOIDCOnly)
	}
	for key, value := range map[string]string{
		"auth.login_path":      c.LoginPath,
		"auth.callback_path":   c.CallbackPath,
		"auth.logout_path":     c.LogoutPath,
		"auth.post_login_path": c.PostLoginPath,
	} {
		if !strings.HasPrefix(value, "/") || strings.Contains(value, "//") {
			return fmt.Errorf("%s must be a rooted local path, got %q", key, value)
		}
	}
	switch c.Protection.Unauthenticated {
	case UnauthenticatedRedirect, UnauthenticatedUnauthorized:
	default:
		return fmt.Errorf("auth.protection.unauthenticated must be %q or %q",
			UnauthenticatedRedirect, UnauthenticatedUnauthorized)
	}
	if _, err := compilePatterns(c.Protection.Include); err != nil {
		return fmt.Errorf("auth.protection.include: %w", err)
	}
	if _, err := compilePatterns(c.Protection.Exclude); err != nil {
		return fmt.Errorf("auth.protection.exclude: %w", err)
	}
	return c.OIDC.validate()
}

func (o OIDCConfig) validate() error {
	if o.Issuer == "" || o.ClientID == "" || o.ClientSecret == "" || o.RedirectURL == "" {
		return fmt.Errorf("auth.oidc requires issuer, client_id, client_secret, and redirect_url")
	}
	if !strings.HasPrefix(o.Issuer, "https://") && !o.AllowLoopbackHTTP {
		return fmt.Errorf("auth.oidc.issuer must be https unless auth.oidc.allow_loopback_http is set")
	}
	if !validClaimName(o.IdentityClaim) {
		return fmt.Errorf("auth.oidc.identity_claim %q is not a usable claim name", o.IdentityClaim)
	}
	switch o.Admission {
	case AdmissionAuthenticated:
	case AdmissionRegistered:
		for _, claim := range o.RegisteredClaims {
			if !validClaimName(claim) {
				return fmt.Errorf("auth.oidc.registered_claims contains an invalid claim name %q", claim)
			}
		}
	case AdmissionExisting:
		if o.AutoProvision {
			return fmt.Errorf("auth.oidc.admission %q requires auto_provision = false", AdmissionExisting)
		}
	case AdmissionClaim:
		if o.Claim.Path == "" || len(o.Claim.Values) == 0 {
			return fmt.Errorf("auth.oidc.admission %q requires claim.path and claim.values", AdmissionClaim)
		}
		if o.Claim.Match != MatchAny && o.Claim.Match != MatchAll {
			return fmt.Errorf("auth.oidc.claim.match must be %q or %q", MatchAny, MatchAll)
		}
	default:
		return fmt.Errorf("auth.oidc.admission must be %q, %q, %q, or %q",
			AdmissionAuthenticated, AdmissionClaim, AdmissionRegistered, AdmissionExisting)
	}
	return nil
}

// validClaimName accepts the shape of a top-level claim name. A claim compared
// as an account key is looked up by exact name, so a nested pointer, an empty
// name, or unprintable bytes are configuration mistakes.
func validClaimName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range len(value) {
		c := value[index]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_', c == '-', c == '.':
		default:
			return false
		}
	}
	return true
}
