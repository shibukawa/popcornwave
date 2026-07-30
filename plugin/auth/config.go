package auth

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

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

// Ceremony user verification levels, as WebAuthn names them.
const (
	UserVerificationRequired    = "required"
	UserVerificationPreferred   = "preferred"
	UserVerificationDiscouraged = "discouraged"
)

// Discoverable credential requirements.
const (
	DiscoverableRequired  = "required"
	DiscoverablePreferred = "preferred"
)

// Registration policies. They decide how an account comes into existence in a
// deployment that does not get one from an identity provider.
const (
	RegistrationDisabled      = "disabled"
	RegistrationOIDC          = "oidc"
	RegistrationInvite        = "invite"
	RegistrationAdministrator = "administrator"
	RegistrationOpen          = "open"
)

// Recovery authorities. A deployment names one before it enables registration,
// because an unrecoverable account is a support incident rather than a policy.
const (
	RecoveryOIDC          = "oidc"
	RecoveryAdministrator = "administrator"
	RecoveryApplication   = "application"
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
	// RecentAuthMaxAge bounds how long a completed authentication still counts
	// as recent enough to add or remove a login method.
	RecentAuthMaxAge time.Duration
	Protection       ProtectionConfig
	Registration     RegistrationConfig
	Recovery         RecoveryConfig
	Bootstrap        BootstrapConfig
	OIDC             OIDCConfig
	Passkey          PasskeyConfig
}

// RegistrationConfig names how a deployment admits a new account.
type RegistrationConfig struct {
	Policy string
}

// RecoveryConfig names the authority that restores access to an account whose
// credentials are gone.
type RecoveryConfig struct {
	Policy string
}

// BootstrapConfig bounds the issued credential that opens a first passkey
// enrollment. See policy:bootstrap-credential-security.
//
// The two durations bound consecutive phases and are deliberately different
// lengths: one covers a person receiving a secret out of band, the other covers
// them finishing a ceremony at the keyboard.
type BootstrapConfig struct {
	// IssueTTL is how long an issued secret stays redeemable, measured from
	// issuance. It spans delivery, so it is the longer of the two.
	IssueTTL time.Duration
	// EnrollmentTTL is how long the enrollment stays open, measured from a
	// successful redemption. It spans one ceremony, so it is short.
	//
	// It is not a session lifetime: what a redemption grants is a ticket that
	// authorizes exactly one registration, and the request stays unauthenticated
	// until that registration finishes.
	EnrollmentTTL time.Duration
	// MaxAttempts bounds how many redemptions may be tried before the
	// credential is spent, whether or not any of them was close.
	MaxAttempts int
}

// PasskeyConfig is the WebAuthn relying-party registration of this deployment.
type PasskeyConfig struct {
	// Path is the base path of the ceremony endpoints. The five endpoints hang
	// off it, so one setting keeps them consistent and keeps them all reachable
	// past the guard.
	Path string
	// RPID scopes every credential. It is a domain, never an IP literal,
	// because an IP address cannot be an RP ID.
	RPID    string
	RPName  string
	Origins []string
	// UserVerification and Discoverable are the ceremony requirements the
	// relying party asks the authenticator for.
	UserVerification string
	Discoverable     string
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
		"auth.recent_auth_max_age":        "5m",
		// Registration and recovery carry no default, because a mode that needs
		// them must not inherit an answer nobody chose.
		"auth.registration.policy":       "",
		"auth.recovery.policy":           "",
		"auth.bootstrap.issue_ttl":       "24h",
		"auth.bootstrap.enrollment_ttl":  "10m",
		"auth.bootstrap.max_attempts":    "5",
		"auth.passkey.path":              "/auth/passkey",
		"auth.passkey.rp_id":             "",
		"auth.passkey.rp_name":           "",
		"auth.passkey.user_verification": UserVerificationRequired,
		"auth.passkey.discoverable":      DiscoverablePreferred,
		"auth.oidc.issuer":               "",
		"auth.oidc.client_id":            "",
		"auth.oidc.client_secret":        "",
		"auth.oidc.redirect_url":         "",
		"auth.oidc.identity_claim":       ClaimSubject,
		"auth.oidc.admission":            AdmissionAuthenticated,
		"auth.oidc.auto_provision":       "true",
		"auth.oidc.claim.path":           "",
		"auth.oidc.claim.match":          MatchAny,
		"auth.oidc.allow_loopback_http":  "false",
		"auth.oidc.provider_logout":      "true",
	}
	keys := []string{
		"auth.enabled", "auth.mode", "auth.login_path", "auth.callback_path",
		"auth.logout_path", "auth.post_login_path", "auth.recent_auth_max_age",
		"auth.protection.include", "auth.protection.exclude", "auth.protection.unauthenticated",
		"auth.registration.policy", "auth.recovery.policy",
		"auth.bootstrap.issue_ttl", "auth.bootstrap.enrollment_ttl", "auth.bootstrap.max_attempts",
		"auth.passkey.path", "auth.passkey.rp_id", "auth.passkey.rp_name", "auth.passkey.origins",
		"auth.passkey.user_verification", "auth.passkey.discoverable",
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
			{Prefix: "auth", Key: "recent_auth_max_age", Help: "how long an authentication stays recent enough to change a login method"},
			{Prefix: "auth", Key: "registration.policy", Help: "disabled, oidc, invite, administrator, or open"},
			{Prefix: "auth", Key: "recovery.policy", Help: "oidc, administrator, or application"},
			{Prefix: "auth", Key: "bootstrap.issue_ttl"},
			{Prefix: "auth", Key: "bootstrap.enrollment_ttl", Help: "how long an enrollment stays open after a redemption"},
			{Prefix: "auth", Key: "bootstrap.max_attempts"},
			{Prefix: "auth", Key: "passkey.path", Help: "base path of the ceremony endpoints"},
			{Prefix: "auth", Key: "passkey.rp_id", Help: "relying party domain; never an IP literal"},
			{Prefix: "auth", Key: "passkey.rp_name"},
			{Prefix: "auth", Key: "passkey.origins", Kind: cliparser.KindArray},
			{Prefix: "auth", Key: "passkey.user_verification", Help: "required, preferred, or discouraged"},
			{Prefix: "auth", Key: "passkey.discoverable", Help: "required or preferred"},
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
			var err error
			if p.RecentAuthMaxAge, err = durationValue(overlay, "auth.recent_auth_max_age"); err != nil {
				return err
			}
			p.Registration = RegistrationConfig{Policy: stringValue(overlay, "auth.registration.policy")}
			p.Recovery = RecoveryConfig{Policy: stringValue(overlay, "auth.recovery.policy")}
			if p.Bootstrap.IssueTTL, err = durationValue(overlay, "auth.bootstrap.issue_ttl"); err != nil {
				return err
			}
			if p.Bootstrap.EnrollmentTTL, err = durationValue(overlay, "auth.bootstrap.enrollment_ttl"); err != nil {
				return err
			}
			if p.Bootstrap.MaxAttempts, err = intValue(overlay, "auth.bootstrap.max_attempts"); err != nil {
				return err
			}
			origins, _ := overlay.GetMulti("auth.passkey.origins")
			p.Passkey = PasskeyConfig{
				Path:             stringValue(overlay, "auth.passkey.path"),
				RPID:             stringValue(overlay, "auth.passkey.rp_id"),
				RPName:           stringValue(overlay, "auth.passkey.rp_name"),
				Origins:          origins,
				UserVerification: stringValue(overlay, "auth.passkey.user_verification"),
				Discoverable:     stringValue(overlay, "auth.passkey.discoverable"),
			}
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
			{Key: "recent_auth_max_age", Kind: configbind.ScaffoldString, Default: "5m", Help: "how long an authentication stays recent enough to change a login method"},
			{Key: "registration.policy", Kind: configbind.ScaffoldString, Default: "", Help: "disabled, oidc, invite, administrator, or open; required unless the mode is oidc_only"},
			{Key: "recovery.policy", Kind: configbind.ScaffoldString, Default: "", Help: "oidc, administrator, or application; required unless the mode is oidc_only"},
			{Key: "bootstrap.issue_ttl", Kind: configbind.ScaffoldString, Default: "24h"},
			{Key: "bootstrap.enrollment_ttl", Kind: configbind.ScaffoldString, Default: "10m", Help: "how long an enrollment stays open after a redemption"},
			{Key: "bootstrap.max_attempts", Kind: configbind.ScaffoldInt, Default: "5"},
			{Key: "passkey.path", Kind: configbind.ScaffoldString, Default: "/auth/passkey", Help: "base path of the ceremony endpoints"},
			{Key: "passkey.rp_id", Kind: configbind.ScaffoldString, Default: "", Help: "relying party domain, such as example.com or localhost; never an IP literal"},
			{Key: "passkey.rp_name", Kind: configbind.ScaffoldString, Default: "", Help: "name the authenticator shows the user"},
			{Key: "passkey.origins", Kind: configbind.ScaffoldStringSlice, Help: "https origins allowed to run a ceremony; loopback http during development"},
			{Key: "passkey.user_verification", Kind: configbind.ScaffoldString, Default: UserVerificationRequired, Help: "required, preferred, or discouraged"},
			{Key: "passkey.discoverable", Kind: configbind.ScaffoldString, Default: DiscoverablePreferred, Help: "required or preferred"},
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
func (c Config) validate() error { return c.validateShape() }

// validateShape applies the rules that outlive the current implementation
// status. It validates only what the selected mode uses and refuses what that
// mode cannot honor, because a silently ignored setting reads as configured
// security.
func (c Config) validateShape() error {
	switch c.Mode {
	case ModeOIDCOnly, ModeOIDCPasskey, ModePasskeyOnly:
	default:
		return fmt.Errorf("auth.mode must be %q, %q, or %q", ModeOIDCOnly, ModeOIDCPasskey, ModePasskeyOnly)
	}
	paths := map[string]string{
		"auth.login_path":      c.LoginPath,
		"auth.callback_path":   c.CallbackPath,
		"auth.logout_path":     c.LogoutPath,
		"auth.post_login_path": c.PostLoginPath,
	}
	if c.usesPasskey() {
		paths["auth.passkey.path"] = c.Passkey.Path
	}
	for key, value := range paths {
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
	if err := c.validateOIDCUse(); err != nil {
		return err
	}
	return c.validatePasskeyUse()
}

func (c Config) usesOIDC() bool    { return c.Mode == ModeOIDCOnly || c.Mode == ModeOIDCPasskey }
func (c Config) usesPasskey() bool { return c.Mode == ModeOIDCPasskey || c.Mode == ModePasskeyOnly }

// issuesBootstrapCredentials reports whether this deployment ever hands out a
// login ID and temporary secret, which is what the bootstrap table stores.
func (c Config) issuesBootstrapCredentials() bool {
	if !c.usesPasskey() {
		return false
	}
	switch c.Registration.Policy {
	case RegistrationInvite, RegistrationAdministrator:
		return true
	}
	// Administrator-reviewed recovery issues one even when registration does
	// not, because that is how a locked-out account gets back in.
	return c.Recovery.Policy == RecoveryAdministrator
}

// validateOIDCUse validates the provider settings in the modes that use them
// and refuses them everywhere else, so a leftover AUTH_OIDC_ISSUER cannot
// suggest that a provider is in the loop.
func (c Config) validateOIDCUse() error {
	if c.usesOIDC() {
		return c.OIDC.validate()
	}
	for key, value := range map[string]string{
		"auth.oidc.issuer":        c.OIDC.Issuer,
		"auth.oidc.client_id":     c.OIDC.ClientID,
		"auth.oidc.client_secret": c.OIDC.ClientSecret,
		"auth.oidc.redirect_url":  c.OIDC.RedirectURL,
	} {
		if value != "" {
			return fmt.Errorf("auth.mode %q reads no OIDC setting, but %s is set", c.Mode, key)
		}
	}
	return nil
}

// validatePasskeyUse validates the relying-party registration and the account
// lifecycle policies the selected mode needs.
func (c Config) validatePasskeyUse() error {
	if !c.usesPasskey() {
		for key, value := range map[string]string{
			"auth.passkey.rp_id":   c.Passkey.RPID,
			"auth.passkey.rp_name": c.Passkey.RPName,
		} {
			if value != "" {
				return fmt.Errorf("auth.mode %q mounts no passkey endpoint, but %s is set", c.Mode, key)
			}
		}
		if len(c.Passkey.Origins) != 0 {
			return fmt.Errorf("auth.mode %q mounts no passkey endpoint, but auth.passkey.origins is set", c.Mode)
		}
		return nil
	}

	if err := c.Passkey.validate(); err != nil {
		return err
	}
	// Enrollment is reachable in every passkey mode, and it is gated on how
	// recently the request proved its identity.
	if c.RecentAuthMaxAge <= 0 {
		return errors.New("auth.recent_auth_max_age must be positive when a passkey may be enrolled")
	}
	// A deployment with two login methods still has to say how a lost
	// credential is restored; one with a single method has to say both.
	if err := validateChoice("auth.recovery.policy", c.Recovery.Policy,
		RecoveryOIDC, RecoveryAdministrator, RecoveryApplication); err != nil {
		return err
	}
	if c.Mode == ModeOIDCPasskey {
		if c.Registration.Policy == "" {
			return nil
		}
		return validateChoice("auth.registration.policy", c.Registration.Policy,
			RegistrationDisabled, RegistrationOIDC, RegistrationInvite, RegistrationAdministrator, RegistrationOpen)
	}
	if c.Recovery.Policy == RecoveryOIDC {
		return fmt.Errorf("auth.recovery.policy %q needs an identity provider, which auth.mode %q has none of",
			RecoveryOIDC, ModePasskeyOnly)
	}
	if err := validateChoice("auth.registration.policy", c.Registration.Policy,
		RegistrationDisabled, RegistrationInvite, RegistrationAdministrator, RegistrationOpen); err != nil {
		return err
	}
	switch c.Registration.Policy {
	case RegistrationInvite, RegistrationAdministrator:
		return c.Bootstrap.validate()
	}
	return nil
}

// validate bounds the issued credential that opens a first enrollment.
func (b BootstrapConfig) validate() error {
	if b.IssueTTL <= 0 {
		return errors.New("auth.bootstrap.issue_ttl must be positive")
	}
	if b.EnrollmentTTL <= 0 {
		return errors.New("auth.bootstrap.enrollment_ttl must be positive")
	}
	if b.MaxAttempts <= 0 {
		return errors.New("auth.bootstrap.max_attempts must be positive")
	}
	return nil
}

func (p PasskeyConfig) validate() error {
	if !validRPID(p.RPID) {
		return fmt.Errorf("auth.passkey.rp_id %q must be a domain such as example.com or localhost, never an IP address", p.RPID)
	}
	if p.RPName == "" {
		return errors.New("auth.passkey.rp_name is required, because an authenticator shows it to the user")
	}
	if len(p.Origins) == 0 {
		return errors.New("auth.passkey.origins requires at least one origin")
	}
	for _, origin := range p.Origins {
		if err := validateOrigin(origin, p.RPID); err != nil {
			return err
		}
	}
	if err := validateChoice("auth.passkey.user_verification", p.UserVerification,
		UserVerificationRequired, UserVerificationPreferred, UserVerificationDiscouraged); err != nil {
		return err
	}
	return validateChoice("auth.passkey.discoverable", p.Discoverable,
		DiscoverableRequired, DiscoverablePreferred)
}

func validateChoice(key, value string, allowed ...string) error {
	if slices.Contains(allowed, value) {
		return nil
	}
	if value == "" {
		return fmt.Errorf("%s is required; choose one of %s", key, strings.Join(allowed, ", "))
	}
	return fmt.Errorf("%s must be one of %s, got %q", key, strings.Join(allowed, ", "), value)
}

// validRPID accepts a registrable domain shape. An IP literal is refused
// because WebAuthn scopes a credential to a domain, so an address can never be
// an RP ID even when it reaches a working server.
func validRPID(value string) bool {
	if value == "" || len(value) > 253 || net.ParseIP(value) != nil {
		return false
	}
	for label := range strings.SplitSeq(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for index := range len(label) {
			c := label[index]
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-':
			default:
				return false
			}
		}
	}
	return true
}

// validateOrigin requires an https origin, or a loopback http origin for local
// development, whose host is the RP ID or a subdomain of it. A credential is
// scoped by the RP ID, so an origin outside that scope could never use it.
func validateOrigin(raw, rpID string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("auth.passkey.origins entry %q must be a bare scheme and host", raw)
	}
	host := parsed.Hostname()
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(host) {
			return fmt.Errorf("auth.passkey.origins entry %q must be https unless its host is loopback", raw)
		}
	default:
		return fmt.Errorf("auth.passkey.origins entry %q must be https or loopback http", raw)
	}
	if host != rpID && !strings.HasSuffix(host, "."+rpID) {
		return fmt.Errorf("auth.passkey.origins entry %q is outside auth.passkey.rp_id %q", raw, rpID)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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

func durationValue(overlay *configbind.Overlay, key string) (time.Duration, error) {
	raw, _ := overlay.GetString(key)
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 5m, got %q", key, raw)
	}
	return parsed, nil
}

func intValue(overlay *configbind.Overlay, key string) (int, error) {
	raw, _ := overlay.GetString(key)
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return parsed, nil
}
