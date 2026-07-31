package auth

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
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
	Enabled bool   `default:"false"`
	Mode    string `default:"oidc_only" dependon:".enabled" help:"oidc_only"`
	// LoginPath starts the provider flow.
	LoginPath    string `default:"/auth/login" dependon:".enabled" help:"path that starts the provider flow"`
	CallbackPath string `default:"/auth/callback" dependon:".enabled"`
	LogoutPath   string `default:"/auth/logout" dependon:".enabled"`
	// PostLoginPath is the local path a completed login lands on.
	PostLoginPath string `default:"/" dependon:".enabled" help:"path a completed login lands on"`
	// RecentAuthMaxAge bounds how long a completed authentication still counts
	// as recent enough to add or remove a login method.
	RecentAuthMaxAge time.Duration      `default:"5m" dependon:".enabled" help:"how recently a request must have authenticated to change a login method"`
	Protection       ProtectionConfig   `dependon:".enabled"`
	Registration     RegistrationConfig `dependon:".enabled"`
	Recovery         RecoveryConfig     `dependon:".enabled"`
	Bootstrap        BootstrapConfig    `dependon:".enabled"`
	OIDC             OIDCConfig         `dependon:".enabled"`
	Passkey          PasskeyConfig      `dependon:".enabled"`
}

// RegistrationConfig names how a deployment admits a new account. It carries no
// default, because a mode that needs it must not inherit an answer nobody chose.
type RegistrationConfig struct {
	Policy string `help:"disabled, oidc, invite, administrator, or open"`
}

// RecoveryConfig names the authority that restores access to an account whose
// credentials are gone.
type RecoveryConfig struct {
	Policy string `help:"oidc, administrator, or application"`
}

// BootstrapConfig bounds the issued credential that opens a first passkey
// enrollment. See policy:bootstrap-credential-security.
//
// The two durations bound consecutive phases and are deliberately different
// lengths: one covers a person receiving a secret out of band, the other covers
// them finishing a ceremony at the keyboard. Neither is a secret, so both are
// shown in the startup summary despite sitting under a credential heading.
type BootstrapConfig struct {
	// IssueTTL is how long an issued secret stays redeemable, measured from
	// issuance. It spans delivery, so it is the longer of the two.
	IssueTTL time.Duration `default:"24h" secret:"show" help:"how long an issued secret stays redeemable"`
	// EnrollmentTTL is how long the enrollment stays open, measured from a
	// successful redemption. It spans one ceremony, so it is short.
	//
	// It is not a session lifetime: what a redemption grants is a ticket that
	// authorizes exactly one registration, and the request stays unauthenticated
	// until that registration finishes.
	EnrollmentTTL time.Duration `default:"10m" secret:"show" help:"how long an enrollment stays open after a redemption"`
	// MaxAttempts bounds how many redemptions may be tried before the
	// credential is spent, whether or not any of them was close.
	MaxAttempts int `default:"5" secret:"show" help:"redemption attempts before the credential is spent"`
}

// PasskeyConfig is the WebAuthn relying-party registration of this deployment.
type PasskeyConfig struct {
	// Path is the base path of the ceremony endpoints. The five endpoints hang
	// off it, so one setting keeps them consistent and keeps them all reachable
	// past the guard.
	Path string `default:"/auth/passkey" help:"base path of the ceremony endpoints"`
	// RPID scopes every credential. It is a domain, never an IP literal,
	// because an IP address cannot be an RP ID.
	RPID    string   `key:"rp_id" help:"relying party domain; localhost during development"`
	RPName  string   `key:"rp_name" help:"relying party display name"`
	Origins []string `help:"origin the browser reaches this deployment on"`
	// UserVerification and Discoverable are the ceremony requirements the
	// relying party asks the authenticator for.
	UserVerification string `default:"required" help:"required, preferred, or discouraged"`
	Discoverable     string `default:"preferred" help:"required or preferred"`
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
