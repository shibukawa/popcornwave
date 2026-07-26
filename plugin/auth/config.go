package auth

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
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
	Enabled bool
	Mode    string
	// LoginPath starts the provider flow. PostLoginPath is the local path a
	// completed login lands on.
	LoginPath     string
	CallbackPath  string
	LogoutPath    string
	PostLoginPath string
	Protection    ProtectionConfig
	OIDC          OIDCConfig
}

// ProtectionConfig selects the paths that require an authenticated request.
type ProtectionConfig struct {
	Include []string
	Exclude []string
	// Unauthenticated is redirect or unauthorized.
	Unauthenticated string
}

// OIDCConfig describes the relying-party registration and admission policy.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
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
	IdentityClaim string
	Admission     string
	// AutoProvision permits an unknown verified identity to create an account
	// through the registered account resolver.
	AutoProvision bool
	Claim         ClaimConfig
	// RegisteredClaims names the verified claims compared against the
	// allowlist table under AdmissionRegistered. It defaults to IdentityClaim
	// alone, because that is the value a deployment registers in advance.
	RegisteredClaims []string
	// ProviderLogout ends the provider session as well, through the
	// discovered end session endpoint. Without it the provider stays signed
	// in, so the next login returns the same user without asking and the
	// sign-out looks like it did nothing.
	ProviderLogout bool
	// AllowLoopbackHTTP permits an http issuer on localhost. It exists for
	// local development against a loopback identity provider and must stay
	// false everywhere else.
	AllowLoopbackHTTP bool
}

// ClaimConfig is the admission rule of AdmissionClaim.
type ClaimConfig struct {
	// Path is a JSON Pointer into the verified ID Token claims.
	Path   string
	Values []string
	Match  string
}

func init() {
	registerConfig()
	pw.RegisterConfig[Config]("auth")
}

func registerConfig() {
	const typeName = "github.com/shibukawa/popcornwave/plugin/auth.Config"
	// List-valued keys carry no scalar default.
	defaults := map[string]string{
		"auth.enabled":                    "false",
		"auth.mode":                       ModeOIDCOnly,
		"auth.login_path":                 "/auth/login",
		"auth.callback_path":              "/auth/callback",
		"auth.logout_path":                "/auth/logout",
		"auth.post_login_path":            "/",
		"auth.protection.unauthenticated": UnauthenticatedRedirect,
		"auth.oidc.issuer":                "",
		"auth.oidc.client_id":             "",
		"auth.oidc.client_secret":         "",
		"auth.oidc.redirect_url":          "",
		"auth.oidc.identity_claim":        ClaimSubject,
		"auth.oidc.admission":             AdmissionAuthenticated,
		"auth.oidc.auto_provision":        "true",
		"auth.oidc.claim.path":            "",
		"auth.oidc.claim.match":           MatchAny,
		"auth.oidc.allow_loopback_http":   "false",
		"auth.oidc.provider_logout":       "true",
	}
	keys := []string{
		"auth.enabled", "auth.mode", "auth.login_path", "auth.callback_path",
		"auth.logout_path", "auth.post_login_path",
		"auth.protection.include", "auth.protection.exclude", "auth.protection.unauthenticated",
		"auth.oidc.issuer", "auth.oidc.client_id", "auth.oidc.client_secret",
		"auth.oidc.redirect_url", "auth.oidc.scopes", "auth.oidc.identity_claim",
		"auth.oidc.admission",
		"auth.oidc.auto_provision", "auth.oidc.claim.path", "auth.oidc.claim.values",
		"auth.oidc.claim.match", "auth.oidc.allow_loopback_http",
		"auth.oidc.provider_logout", "auth.oidc.registered_claims",
	}
	sort.Strings(keys)
	configbind.Register[Config](configbind.Definition{
		TypeName:  typeName,
		Prefix:    "auth",
		KnownKeys: keys,
		Defaults:  defaults,
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "auth", Key: "enabled", Kind: cliparser.KindBool},
			{Prefix: "auth", Key: "mode", Help: "oidc_only"},
			{Prefix: "auth", Key: "login_path"},
			{Prefix: "auth", Key: "callback_path"},
			{Prefix: "auth", Key: "logout_path"},
			{Prefix: "auth", Key: "post_login_path"},
			{Prefix: "auth", Key: "protection.include", Kind: cliparser.KindArray, Help: "protected path pattern"},
			{Prefix: "auth", Key: "protection.exclude", Kind: cliparser.KindArray, Help: "public path pattern"},
			{Prefix: "auth", Key: "protection.unauthenticated", Help: "redirect or unauthorized"},
			{Prefix: "auth", Key: "oidc.issuer", Env: "AUTH_OIDC_ISSUER"},
			{Prefix: "auth", Key: "oidc.client_id", Env: "AUTH_OIDC_CLIENT_ID"},
			{Prefix: "auth", Key: "oidc.client_secret", Env: "AUTH_OIDC_CLIENT_SECRET"},
			{Prefix: "auth", Key: "oidc.redirect_url"},
			{Prefix: "auth", Key: "oidc.scopes", Kind: cliparser.KindArray},
			{Prefix: "auth", Key: "oidc.identity_claim", Help: "verified claim that identifies a local account"},
			{Prefix: "auth", Key: "oidc.admission", Help: "authenticated, claim, registered, or existing"},
			{Prefix: "auth", Key: "oidc.auto_provision", Kind: cliparser.KindBool},
			{Prefix: "auth", Key: "oidc.claim.path", Help: "JSON Pointer into verified claims"},
			{Prefix: "auth", Key: "oidc.claim.values", Kind: cliparser.KindArray},
			{Prefix: "auth", Key: "oidc.claim.match", Help: "any or all"},
			{Prefix: "auth", Key: "oidc.provider_logout", Kind: cliparser.KindBool, Help: "also end the provider session on logout"},
			{Prefix: "auth", Key: "oidc.allow_loopback_http", Kind: cliparser.KindBool, Help: "permit an http loopback issuer during development"},
			{Prefix: "auth", Key: "oidc.registered_claims", Kind: cliparser.KindArray, Help: "claims compared against the allowlist; defaults to identity_claim"},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			p, ok := dst.(*Config)
			if !ok || p == nil {
				return fmt.Errorf("configbind: bad auth.Config destination")
			}
			p.Enabled = boolValue(overlay, "auth.enabled")
			p.Mode = stringValue(overlay, "auth.mode")
			p.LoginPath = stringValue(overlay, "auth.login_path")
			p.CallbackPath = stringValue(overlay, "auth.callback_path")
			p.LogoutPath = stringValue(overlay, "auth.logout_path")
			p.PostLoginPath = stringValue(overlay, "auth.post_login_path")
			include, _ := overlay.GetMulti("auth.protection.include")
			exclude, _ := overlay.GetMulti("auth.protection.exclude")
			p.Protection = ProtectionConfig{
				Include:         include,
				Exclude:         exclude,
				Unauthenticated: stringValue(overlay, "auth.protection.unauthenticated"),
			}
			scopes, _ := overlay.GetMulti("auth.oidc.scopes")
			values, _ := overlay.GetMulti("auth.oidc.claim.values")
			registered, _ := overlay.GetMulti("auth.oidc.registered_claims")
			p.OIDC = OIDCConfig{
				Issuer:            stringValue(overlay, "auth.oidc.issuer"),
				ClientID:          stringValue(overlay, "auth.oidc.client_id"),
				ClientSecret:      stringValue(overlay, "auth.oidc.client_secret"),
				RedirectURL:       stringValue(overlay, "auth.oidc.redirect_url"),
				Scopes:            scopes,
				IdentityClaim:     stringValue(overlay, "auth.oidc.identity_claim"),
				Admission:         stringValue(overlay, "auth.oidc.admission"),
				AutoProvision:     boolValue(overlay, "auth.oidc.auto_provision"),
				AllowLoopbackHTTP: boolValue(overlay, "auth.oidc.allow_loopback_http"),
				ProviderLogout:    boolValue(overlay, "auth.oidc.provider_logout"),
				RegisteredClaims:  registered,
				Claim: ClaimConfig{
					Path:   stringValue(overlay, "auth.oidc.claim.path"),
					Values: values,
					Match:  stringValue(overlay, "auth.oidc.claim.match"),
				},
			}
			return nil
		},
		Scaffold: []configbind.ScaffoldField{
			{Key: "enabled", Kind: configbind.ScaffoldBool, Default: "false"},
			{Key: "mode", Kind: configbind.ScaffoldString, Default: ModeOIDCOnly, Help: "oidc_only"},
			{Key: "login_path", Kind: configbind.ScaffoldString, Default: "/auth/login"},
			{Key: "callback_path", Kind: configbind.ScaffoldString, Default: "/auth/callback"},
			{Key: "logout_path", Kind: configbind.ScaffoldString, Default: "/auth/logout"},
			{Key: "post_login_path", Kind: configbind.ScaffoldString, Default: "/"},
			{Key: "protection.include", Kind: configbind.ScaffoldStringSlice, Help: "protected path pattern such as /account or /admin/**"},
			{Key: "protection.exclude", Kind: configbind.ScaffoldStringSlice, Help: "public override; security sensitive"},
			{Key: "protection.unauthenticated", Kind: configbind.ScaffoldString, Default: UnauthenticatedRedirect, Help: "redirect or unauthorized"},
			{Key: "oidc.issuer", Kind: configbind.ScaffoldString, Default: "", Env: "AUTH_OIDC_ISSUER"},
			{Key: "oidc.client_id", Kind: configbind.ScaffoldString, Default: "", Env: "AUTH_OIDC_CLIENT_ID"},
			{Key: "oidc.client_secret", Kind: configbind.ScaffoldString, Default: "", Env: "AUTH_OIDC_CLIENT_SECRET"},
			{Key: "oidc.redirect_url", Kind: configbind.ScaffoldString, Default: ""},
			{Key: "oidc.scopes", Kind: configbind.ScaffoldStringSlice, Help: "additional scopes; openid is always requested"},
			{Key: "oidc.identity_claim", Kind: configbind.ScaffoldString, Default: ClaimSubject, Help: "verified claim that identifies a local account; must be stable and unique per issuer"},
			{Key: "oidc.admission", Kind: configbind.ScaffoldString, Default: AdmissionAuthenticated, Help: "authenticated, claim, registered, or existing"},
			{Key: "oidc.auto_provision", Kind: configbind.ScaffoldBool, Default: "true"},
			{Key: "oidc.claim.path", Kind: configbind.ScaffoldString, Default: "", Help: "JSON Pointer into verified claims"},
			{Key: "oidc.claim.values", Kind: configbind.ScaffoldStringSlice},
			{Key: "oidc.claim.match", Kind: configbind.ScaffoldString, Default: MatchAny, Help: "any or all"},
			{Key: "oidc.provider_logout", Kind: configbind.ScaffoldBool, Default: "true", Help: "also end the provider session on logout"},
			{Key: "oidc.allow_loopback_http", Kind: configbind.ScaffoldBool, Default: "false", Help: "permit an http loopback issuer during development"},
			{Key: "oidc.registered_claims", Kind: configbind.ScaffoldStringSlice, Help: "claims compared against the allowlist under registered admission; defaults to identity_claim"},
		},
	})
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

func stringValue(overlay *configbind.Overlay, key string) string {
	value, _ := overlay.GetString(key)
	return value
}

func boolValue(overlay *configbind.Overlay, key string) bool {
	value, _ := overlay.GetString(key)
	return value == "true" || value == "1"
}
