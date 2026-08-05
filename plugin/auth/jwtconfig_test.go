package auth

import (
	"strings"
	"testing"
	"time"
)

// validJWTConfig is the smallest configuration that serves. Each test below
// breaks exactly one field of it, so a failure names the rule it broke.
func validJWTConfig() Config {
	return Config{
		Enabled:       true,
		Mode:          ModeJWTOnly,
		LoginPath:     "/auth/login",
		CallbackPath:  "/auth/callback",
		LogoutPath:    "/auth/logout",
		PostLoginPath: "/",
		Protection: ProtectionConfig{
			Include:         []string{"/api/*"},
			Unauthenticated: UnauthenticatedUnauthorized,
		},
		JWT: JWTConfig{
			Issuer:              "https://issuer.example",
			Audience:            []string{"https://api.example"},
			AudienceMatch:       MatchAny,
			Algorithms:          []string{"RS256"},
			RequiredTokenType:   accessTokenType,
			Discovery:           DiscoveryOIDC,
			Leeway:              30 * time.Second,
			MaxTokenLifetime:    time.Hour,
			MaxTokenBytes:       8192,
			JWKSRefreshCooldown: time.Minute,
			IdentityClaim:       ClaimSubject,
			Admission:           AdmissionAuthenticated,
			Revocation: JWTRevocationConfig{
				Mode:          RevocationBoth,
				OnUnavailable: RevocationRefuse,
			},
		},
	}
}

func TestJWTOnlyAcceptsAValidConfiguration(t *testing.T) {
	if err := validJWTConfig().validate(); err != nil {
		t.Fatalf("valid jwt_only configuration rejected: %v", err)
	}
}

// The fields with a permissive answer carry no default, so forgetting one has
// to fail at startup rather than resolve to the answer nobody chose.
func TestJWTOnlyRequiresEveryFieldWithAPermissiveAnswer(t *testing.T) {
	for name, breakIt := range map[string]func(*Config){
		"audience":           func(c *Config) { c.JWT.Audience = nil },
		"admission":          func(c *Config) { c.JWT.Admission = "" },
		"max_token_lifetime": func(c *Config) { c.JWT.MaxTokenLifetime = 0 },
		"revocation.mode":    func(c *Config) { c.JWT.Revocation.Mode = "" },
		"algorithms":         func(c *Config) { c.JWT.Algorithms = nil },
		"issuer":             func(c *Config) { c.JWT.Issuer = "" },
	} {
		t.Run(name, func(t *testing.T) {
			config := validJWTConfig()
			breakIt(&config)
			err := config.validate()
			if err == nil {
				t.Fatalf("missing %s was accepted", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("error for missing %s does not name the key: %v", name, err)
			}
		})
	}
}

// A published JWKS is a public document. Accepting a symmetric algorithm
// against one would let anyone holding it sign their own tokens.
func TestJWTOnlyRefusesSymmetricAlgorithms(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Algorithms = []string{"HS256"}
	err := config.validate()
	if err == nil {
		t.Fatal("HS256 was accepted against a public key set")
	}
	if !strings.Contains(err.Error(), "mint tokens") {
		t.Fatalf("error does not explain the consequence: %v", err)
	}
}

func TestJWTOnlyRefusesAlgorithmNone(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Algorithms = []string{"none"}
	if err := config.validate(); err == nil {
		t.Fatal(`"none" was accepted in the algorithm allowlist`)
	}
}

// An issuer reachable over plain http can be rewritten in flight, which makes
// every check that follows it decorative.
func TestJWTOnlyRequiresAnHTTPSIssuerOutsideLoopback(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Issuer = "http://issuer.example"
	if err := config.validate(); err == nil {
		t.Fatal("plain http issuer was accepted")
	}

	config.JWT.Issuer = "http://127.0.0.1:8080"
	config.JWT.AllowLoopbackHTTP = true
	if err := config.validate(); err != nil {
		t.Fatalf("loopback http issuer rejected under the development allowance: %v", err)
	}

	// The allowance is for loopback, not for http generally.
	config.JWT.Issuer = "http://issuer.example"
	if err := config.validate(); err == nil {
		t.Fatal("allow_loopback_http accepted a non-loopback http issuer")
	}
}

// A key set on another origin would mean whoever controls that host controls
// which signatures verify here.
func TestJWTOnlyRequiresTheKeySetOnTheIssuerOrigin(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Discovery = DiscoveryManual
	config.JWT.JWKSURI = "https://elsewhere.example/keys"
	err := config.validate()
	if err == nil {
		t.Fatal("a key set on another origin was accepted")
	}
	if !strings.Contains(err.Error(), "decorative") {
		t.Fatalf("error does not explain the consequence: %v", err)
	}

	config.JWT.JWKSURI = "https://issuer.example/keys"
	if err := config.validate(); err != nil {
		t.Fatalf("same-origin key set rejected: %v", err)
	}
}

func TestJWTOnlyRefusesAJWKSURIOutsideManualDiscovery(t *testing.T) {
	config := validJWTConfig()
	config.JWT.JWKSURI = "https://issuer.example/keys"
	if err := config.validate(); err == nil {
		t.Fatal("jwks_uri was accepted under metadata discovery, where nothing reads it")
	}
}

// An API has no login page, so a redirect would be answered by a client that
// cannot render one.
func TestJWTOnlyRequiresAnUnauthorizedResponse(t *testing.T) {
	config := validJWTConfig()
	config.Protection.Unauthenticated = UnauthenticatedRedirect
	if err := config.validate(); err == nil {
		t.Fatal("redirect response was accepted for a bearer API")
	}
}

// A cache that outlives every token it could refuse is a cache that never
// expires a wrong answer.
func TestJWTOnlyBoundsTheRevocationCacheByTokenLifetime(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Revocation.MaxPropagationDelay = config.JWT.MaxTokenLifetime
	if err := config.validate(); err == nil {
		t.Fatal("a propagation delay as long as a token lifetime was accepted")
	}

	config.JWT.Revocation.MaxPropagationDelay = time.Minute
	if err := config.validate(); err != nil {
		t.Fatalf("a bounded propagation delay was rejected: %v", err)
	}
}

// A propagation delay means nothing when nothing is being revoked, and a
// setting that is read in one mode and ignored in another reads as configured
// behavior.
func TestJWTOnlyRefusesACacheSettingWithRevocationOff(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Revocation.Mode = RevocationOff
	config.JWT.Revocation.MaxPropagationDelay = time.Minute
	if err := config.validate(); err == nil {
		t.Fatal("a propagation delay was accepted with revocation off")
	}
}

func TestJWTOnlyRefusesAutoProvisionUnderExistingAdmission(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Admission = AdmissionExisting
	config.JWT.AutoProvision = true
	if err := config.validate(); err == nil {
		t.Fatal("existing admission accepted auto_provision, which it forbids")
	}
}

func TestJWTOnlyRequiresAClaimRuleUnderClaimAdmission(t *testing.T) {
	config := validJWTConfig()
	config.JWT.Admission = AdmissionClaim
	if err := config.validate(); err == nil {
		t.Fatal("claim admission was accepted with no rule to apply")
	}

	config.JWT.Claim = ClaimConfig{Path: "/groups", Values: []string{"eng"}, Match: MatchAny}
	if err := config.validate(); err != nil {
		t.Fatalf("claim admission with a rule was rejected: %v", err)
	}
}

// A leftover AUTH_JWT_ISSUER must not suggest that a browser deployment accepts
// bearer tokens.
func TestOtherModesRefuseBearerSettings(t *testing.T) {
	config := Config{
		Enabled: true, Mode: ModeOIDCOnly,
		LoginPath: "/auth/login", CallbackPath: "/auth/callback",
		LogoutPath: "/auth/logout", PostLoginPath: "/",
		Protection: ProtectionConfig{Unauthenticated: UnauthenticatedRedirect},
		OIDC: OIDCConfig{
			Issuer: "https://issuer.example", ClientID: "id", ClientSecret: "secret",
			RedirectURL: "https://app.example/auth/callback",
			Admission:   AdmissionAuthenticated, IdentityClaim: ClaimSubject,
			LogoutScope: LogoutScopeReconfirm,
		},
		JWT: JWTConfig{Issuer: "https://issuer.example"},
	}
	err := config.validate()
	if err == nil {
		t.Fatal("oidc_only accepted auth.jwt.issuer")
	}
	if !strings.Contains(err.Error(), "auth.jwt.issuer") {
		t.Fatalf("error does not name the offending key: %v", err)
	}
}

// jwt_only reads no provider or relying-party setting, and the existing
// refusals must cover it rather than only the modes they were written for.
func TestJWTOnlyRefusesOIDCAndPasskeySettings(t *testing.T) {
	config := validJWTConfig()
	config.OIDC.Issuer = "https://issuer.example"
	if err := config.validate(); err == nil {
		t.Fatal("jwt_only accepted an OIDC issuer")
	}

	config = validJWTConfig()
	config.Passkey.RPID = "app.example"
	if err := config.validate(); err == nil {
		t.Fatal("jwt_only accepted a passkey relying-party registration")
	}
}

// The token form names a token by its jti, so a deployment that relaxed the
// token type still has to demand one.
func TestTokenRevocationRequiresAJTIEvenWithARelaxedTokenType(t *testing.T) {
	config := validJWTConfig()
	config.JWT.RequiredTokenType = ""
	config.JWT.Revocation.Mode = RevocationSubject
	if config.JWT.requiresJTI() {
		t.Fatal("jti demanded with no token type and no token-form revocation")
	}

	config.JWT.Revocation.Mode = RevocationToken
	if !config.JWT.requiresJTI() {
		t.Fatal("token-form revocation did not demand a jti")
	}
}
