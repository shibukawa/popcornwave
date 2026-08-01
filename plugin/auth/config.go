package auth

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/session"
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
	RecentAuthMaxAge time.Duration `default:"5m" dependon:".enabled" help:"how recently a request must have authenticated to change a login method"`
	// SharedDevice declares that the browsers reaching this deployment are
	// shared, which couples the settings that would otherwise leave one user
	// visible to the next. It fixes the logout scope to global and withholds
	// the select_account prompt, because that prompt exists to surface exactly
	// what this mode hides.
	//
	// Any one of those alone achieves nothing: with the local hint disabled
	// but the provider session alive, the next visitor still sees the previous
	// account in the provider's own account picker.
	//
	// It reduces disclosure and does not eliminate it. The common end of a
	// session on a shared device is abandonment rather than logout, and no
	// relying party can end a provider session it was not asked to end.
	SharedDevice bool               `default:"false" dependon:".enabled" help:"declare that browsers are shared, coupling the settings that hide one user from the next"`
	Assurance    AssuranceConfig    `dependon:".enabled"`
	Protection   ProtectionConfig   `dependon:".enabled"`
	Registration RegistrationConfig `dependon:".enabled"`
	Recovery     RecoveryConfig     `dependon:".enabled"`
	Bootstrap    BootstrapConfig    `dependon:".enabled"`
	OIDC         OIDCConfig         `dependon:".enabled"`
	Passkey      PasskeyConfig      `dependon:".enabled"`
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
	// LogoutScope decides what a logout does to the session the provider
	// holds, which is not the session this application owns.
	//
	// reconfirm revokes the local session, sends the provider nothing, and
	// marks the next authorization to carry prompt, so the provider still
	// demands proof while every other relying party sharing it is untouched.
	//
	// global additionally ends the provider session through the discovered end
	// session endpoint, which signs the user out of every application sharing
	// that provider. It is what a shared device wants and what a personal one
	// rarely does.
	//
	// There is no local-only value: revoking only the local session leaves the
	// next login silent, so the sign-out looks like it did nothing. That was
	// the failure reconfirm exists to fix.
	LogoutScope string `default:"reconfirm" enum:"reconfirm,global" help:"what a logout does to the provider session: reconfirm or global"`
	// ProviderLogout is removed and survives only to fail loudly. configbind
	// ignores a key no field declares, so deleting the field outright would
	// leave every scaffolded project silently running reconfirm while its
	// configuration still read as a global sign-out.
	//
	// The default is inverted to false so presence is detectable: an untouched
	// project binds false and starts, and one carrying provider_logout = true
	// is refused with LogoutScope named. A leftover false meant the rejected
	// local scope, whose nearest surviving behavior is the new default.
	//
	// Delete this once no configuration in the wild carries the key.
	ProviderLogout bool `default:"false" help:"removed; use auth.oidc.logout_scope"`
	// AllowGlobalLogoutRequest lets the logout request escalate to global, for
	// a deployment offering both a sign-out and a sign-out-everywhere control.
	// A request may only escalate: a forced downgrade would leave the provider
	// session alive after the user asked to leave it.
	AllowGlobalLogoutRequest bool `default:"false" help:"permit a logout request to escalate to a global sign-out"`
	// AllowLoopbackHTTP permits an http issuer on localhost. It exists for
	// local development against a loopback identity provider and must stay
	// false everywhere else.
	AllowLoopbackHTTP bool `default:"false" help:"permit an http loopback issuer during development"`
}

// AssuranceConfig holds the named freshness windows a handler requires by
// name, so the same handler code serves a consumer deployment with a long
// window and an internal one with a short window.
type AssuranceConfig struct {
	// Policy is an array of tables rather than a map, because configbind binds
	// statically and a map key is not a declared field:
	//
	//	[[auth.assurance.policy]]
	//	name = "admin"
	//	max_age = "15m"
	Policy   []AssurancePolicy `help:"named freshness windows a handler can require by name"`
	Hint     HintConfig        `help:"what the login screen may remember about the last user of a browser"`
	Presence PresenceConfig    `help:"end a session when nobody is at the keyboard, rather than when no request arrives"`
}

// PresenceConfig turns on the endpoint a browser reports human presence to.
//
// Idle expiry otherwise measures time since the last HTTP request, which is a
// proxy for presence that fails in both directions: a page holding a live
// connection reconnects on its own and keeps an unattended browser signed in,
// while a person reading one page for longer than the timeout issues no request
// at all and is signed out mid-work.
type PresenceConfig struct {
	Enabled bool `default:"false" help:"accept presence reports from the browser"`
	// Interval is how often the browser is expected to report. It bounds the
	// endpoint's rate and sets the pace the scaffolded script ticks at.
	Interval time.Duration `default:"1m" help:"how often a browser reports"`
	// AbsentAfter ends the session once no interaction has been reported for
	// this long. It measures a person rather than a request, which is the whole
	// point of the signal.
	AbsentAfter time.Duration `default:"30m" help:"end the session after this long with no interaction"`
}

// HintConfig controls whether an ended session leaves a non-authoritative note
// of who was signed in, so the next sign-in is shorter.
//
// It is off by default. A deployment that has not thought about shared devices
// therefore drops a browser straight from signed-in to anonymous, where the
// login screen offers no account and no issuer.
//
// The hint grants nothing. A request carrying one is unauthenticated, and the
// path guard denies it exactly as it denies any other.
type HintConfig struct {
	Enabled bool   `default:"false" help:"remember who last signed in, to shorten the next sign-in"`
	Name    string `default:"pw_hint" help:"cookie name"`
	// Secret seals the cookie. The contents never reach the client, so the
	// hint may hold a login identifier; what must not leak is what the login
	// screen renders, which is masked instead.
	Secret string `secret:"mask" env:"AUTH_HINT_SECRET" help:"base64 secret of at least 256 bits that seals the hint"`
	// PreviousSecrets keep a rotation readable.
	PreviousSecrets []string `secret:"mask" help:"retired secrets kept readable during a rotation"`
	// TTL is the absolute bound and IdleTimeout the one measured from the last
	// successful login. The pair is the session's own shape, for the same
	// reasons, and is deliberately not inherited from it: a hint outlives a
	// session by design.
	//
	// Setting TTL to zero is a valid answer. It means this browser may
	// remember nothing, which is what a shared terminal wants and what
	// SharedDevice sets for a whole deployment.
	TTL         time.Duration `default:"720h" help:"how long a hint may live at all"`
	IdleTimeout time.Duration `default:"336h" help:"how long since the last successful login a hint survives"`
}

// AssurancePolicy names one freshness window. A zero MaxAge is meaningful and
// means prove again for this operation; it is not an unset field.
type AssurancePolicy struct {
	Name   string        `help:"name a handler passes to auth.Policy"`
	MaxAge time.Duration `help:"how old a proof may be; zero means prove again for this operation"`
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
	if err := c.validateAssurance(); err != nil {
		return err
	}
	if err := c.validateOIDCUse(); err != nil {
		return err
	}
	return c.validatePasskeyUse()
}

// validateAssurance rejects a policy table a handler could not resolve, so a
// name that does not exist fails at startup rather than at the request that
// needed it. It also refuses the settings a shared-device deployment cannot
// honor, rather than overriding them: a configuration file that reads as one
// behavior while the deployment runs another is the failure this whole mode
// exists to avoid.
func (c Config) validateAssurance() error {
	seen := make(map[string]bool, len(c.Assurance.Policy))
	for i, policy := range c.Assurance.Policy {
		if policy.Name == "" {
			return fmt.Errorf("auth.assurance.policy[%d].name must be set", i)
		}
		if seen[policy.Name] {
			return fmt.Errorf("auth.assurance.policy name %q is declared twice", policy.Name)
		}
		seen[policy.Name] = true
		if policy.MaxAge < 0 {
			return fmt.Errorf("auth.assurance.policy %q: max_age must not be negative", policy.Name)
		}
	}
	if c.RecentAuthMaxAge < 0 {
		return errors.New("auth.recent_auth_max_age must not be negative")
	}
	if c.OIDC.ProviderLogout {
		return fmt.Errorf("auth.oidc.provider_logout is removed; set auth.oidc.logout_scope = %q for the same behavior, or %q to keep the provider session and re-confirm at the next login",
			LogoutScopeGlobal, LogoutScopeReconfirm)
	}
	if c.SharedDevice && c.usesOIDC() && c.OIDC.LogoutScope != LogoutScopeGlobal {
		return fmt.Errorf("auth.shared_device requires auth.oidc.logout_scope %q, got %q",
			LogoutScopeGlobal, c.OIDC.LogoutScope)
	}
	if c.SharedDevice && c.Assurance.Hint.Enabled {
		return errors.New("auth.shared_device forbids auth.assurance.hint.enabled: remembering the last user is what the mode exists to prevent")
	}
	if err := c.Assurance.Hint.validate(); err != nil {
		return err
	}
	return c.Assurance.Presence.validate()
}

func (p PresenceConfig) validate() error {
	if !p.Enabled {
		return nil
	}
	if p.Interval <= 0 {
		return errors.New("auth.assurance.presence.interval must be positive")
	}
	if p.AbsentAfter <= 0 {
		return errors.New("auth.assurance.presence.absent_after must be positive")
	}
	if p.AbsentAfter <= p.Interval {
		// One missed tick would otherwise end the session, which a slow network
		// produces as readily as an empty chair.
		return errors.New("auth.assurance.presence.absent_after must exceed interval, so a single late report does not end a session")
	}
	return nil
}

// validate checks the hint settings of a deployment that turned it on. An
// unusable secret is refused rather than downgraded, because a hint that cannot
// be sealed would have to be written in the clear or silently dropped, and both
// are worse than refusing to start.
func (h HintConfig) validate() error {
	if !h.Enabled {
		return nil
	}
	if h.Name == "" {
		return errors.New("auth.assurance.hint.name must be set")
	}
	if h.TTL < 0 || h.IdleTimeout < 0 {
		return errors.New("auth.assurance.hint.ttl and idle_timeout must not be negative")
	}
	if h.IdleTimeout > h.TTL && h.TTL > 0 {
		return errors.New("auth.assurance.hint.idle_timeout must not exceed ttl, which already bounds it")
	}
	if _, err := session.ParseKeyring(append([]string{h.Secret}, h.PreviousSecrets...)...); err != nil {
		// The error names the key and never repeats the value.
		return fmt.Errorf("auth.assurance.hint.secret: %w", err)
	}
	return nil
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
