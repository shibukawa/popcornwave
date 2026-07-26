package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/contrib/jwt"
)

var (
	// ErrAccessDenied rejects a verified identity that admission or account
	// state does not admit. It is answered with one enumeration-safe response.
	ErrAccessDenied = errors.New("auth: access denied")
	// ErrUnknownIdentity is returned by an account resolver that found no local
	// account for a verified identity.
	ErrUnknownIdentity = errors.New("auth: unknown identity")
)

// Identity is a verified external identity. Every field comes from a verified
// ID Token; the mutable display claims are copied for rendering and never
// identify the account link.
type Identity struct {
	Issuer  string
	Subject string
	// KeyClaim names the claim a deployment identifies accounts by, and Key is
	// its verified value. They default to the "sub" claim and the subject.
	//
	// A directory that issues its own stable identifier, such as an employee
	// number, usually knows that value before anyone logs in, which the
	// subject never is. auth.oidc.identity_claim selects it.
	KeyClaim string
	Key      string
	Claims   Claims
}

// ClaimSubject is the default identity claim and the only one OpenID Connect
// guarantees is stable and unique for an issuer.
const ClaimSubject = "sub"

// maxLookupValueBytes bounds one compared claim value, so a provider cannot
// turn an account lookup into an unbounded query parameter.
const maxLookupValueBytes = 512

// Claims is the read-only verified claim set of an ID Token.
type Claims struct {
	raw map[string]json.RawMessage
}

// String returns a string claim by name.
func (c Claims) String(name string) (string, bool) {
	value, ok := c.raw[name]
	if !ok {
		return "", false
	}
	var result string
	if json.Unmarshal(value, &result) != nil {
		return "", false
	}
	return result, true
}

// Raw returns the undecoded JSON of one claim.
func (c Claims) Raw(name string) (json.RawMessage, bool) {
	value, ok := c.raw[name]
	return value, ok
}

// claimLookupValue reads one claim as an account lookup value. The subject is
// taken from the verified sub claim rather than from the copied claim map.
//
// A string is used as it is. An integer is used as its literal text, because a
// directory that issues numeric employee identifiers often emits them as JSON
// numbers. Every other shape, including a fractional number, is refused rather
// than normalized, so two systems cannot disagree about what the value is.
func claimLookupValue(claim string, identity Identity) (string, bool) {
	if claim == "" {
		return "", false
	}
	value := ""
	switch claim {
	case ClaimSubject:
		value = identity.Subject
	default:
		raw, ok := identity.Claims.raw[claim]
		if !ok {
			return "", false
		}
		var text string
		switch {
		case json.Unmarshal(raw, &text) == nil:
			value = text
		case isIntegerLiteral(raw):
			value = string(raw)
		default:
			return "", false
		}
	}
	if value == "" || len(value) > maxLookupValueBytes {
		return "", false
	}
	return value, true
}

// isIntegerLiteral reports whether raw is a JSON number without a fraction or
// exponent, whose text is therefore an unambiguous identifier.
func isIntegerLiteral(raw []byte) bool {
	if len(raw) == 0 || len(raw) > 64 {
		return false
	}
	index := 0
	if raw[0] == '-' {
		index++
	}
	if index == len(raw) {
		return false
	}
	for ; index < len(raw); index++ {
		if raw[index] < '0' || raw[index] > '9' {
			return false
		}
	}
	return true
}

// Account is the local account a verified identity resolved to. Applications
// own account storage; this is only what the session needs.
type Account struct {
	// ID is the stable opaque application identifier. It must not be an email
	// address.
	ID          string
	DisplayName string
	Email       string
	// Suspended blocks session creation for an account that exists but may not
	// log in.
	Suspended bool
}

// AccountResolver resolves or provisions the local account of a verified
// identity. Returning ErrUnknownIdentity means no local account exists, which
// admission then interprets according to policy.
//
// provision reports whether policy permits creating an account during this
// login.
type AccountResolver func(ctx context.Context, identity Identity, provision bool) (Account, error)

var resolverState struct {
	sync.RWMutex
	resolve AccountResolver
}

// SetAccountResolver installs the application account resolver. Call it from
// main before pw.Run. Without a resolver the framework derives a deterministic
// account identifier from the verified issuer and subject, which suits a
// deployment that keeps no local account table.
func SetAccountResolver(resolve AccountResolver) {
	resolverState.Lock()
	defer resolverState.Unlock()
	resolverState.resolve = resolve
}

func accountResolver() AccountResolver {
	resolverState.RLock()
	defer resolverState.RUnlock()
	if resolverState.resolve != nil {
		return resolverState.resolve
	}
	return derivedAccount
}

// derivedAccount is the default resolver. It never stores anything, so every
// verified identity resolves to the same stable identifier across restarts.
func derivedAccount(_ context.Context, identity Identity, _ bool) (Account, error) {
	sum := sha256.Sum256([]byte(identity.Issuer + "\x00" + identity.KeyClaim + "\x00" + identity.Key))
	account := Account{ID: base64.RawURLEncoding.EncodeToString(sum[:16])}
	if name, ok := identity.Claims.String("name"); ok {
		account.DisplayName = name
	} else if name, ok := identity.Claims.String("preferred_username"); ok {
		account.DisplayName = name
	}
	if email, ok := identity.Claims.String("email"); ok {
		account.Email = email
	}
	return account, nil
}

// SessionData is the payload stored in the login session. It holds no token
// body, no provider secret, and no raw cookie material.
type SessionData struct {
	AccountID string `json:"account_id"`
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	// KeyClaim and Key record which verified claim identified the account and
	// its value, so a handler can show or audit the link without repeating the
	// configuration.
	KeyClaim    string `json:"key_claim,omitempty"`
	Key         string `json:"key,omitempty"`
	DisplayName string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
}

// identityFrom builds the verified identity and resolves the configured lookup
// key. A configured claim that the token does not carry, or carries in an
// unusable shape, leaves Key empty and is refused by admit.
func identityFrom(claims jwt.Claims, identityClaim string) Identity {
	if identityClaim == "" {
		identityClaim = ClaimSubject
	}
	identity := Identity{
		Issuer:   claims.Issuer,
		Subject:  claims.Subject,
		KeyClaim: identityClaim,
		Claims:   Claims{raw: claims.Raw},
	}
	if key, ok := claimLookupValue(identityClaim, identity); ok {
		identity.Key = key
	}
	return identity
}

// admit applies policy:oidc-admission to an already verified identity. It runs
// on every login, including one that resolves to an existing account.
func admit(ctx context.Context, config OIDCConfig, allowlist Allowlist, identity Identity) (Account, error) {
	if identity.Issuer == "" || identity.Subject == "" || identity.Key == "" {
		// No usable lookup key means no account can be identified, so nothing
		// downstream may treat this login as a known person.
		return Account{}, ErrAccessDenied
	}
	switch config.Admission {
	case AdmissionClaim:
		if !claimAdmits(config.Claim, identity.Claims) {
			return Account{}, ErrAccessDenied
		}
	case AdmissionRegistered:
		registered, err := allowlist.registered(ctx, config.RegisteredClaims, identity)
		if err != nil {
			// A lookup failure is an error, never a denial, so an outage
			// cannot silently reopen or close a deployment.
			return Account{}, err
		}
		if !registered {
			return Account{}, ErrAccessDenied
		}
	}
	provision := config.AutoProvision && config.Admission != AdmissionExisting
	account, err := accountResolver()(ctx, identity, provision)
	switch {
	case errors.Is(err, ErrUnknownIdentity):
		return Account{}, ErrAccessDenied
	case err != nil:
		return Account{}, err
	case account.ID == "":
		return Account{}, ErrAccessDenied
	case account.Suspended:
		return Account{}, ErrAccessDenied
	}
	return account, nil
}

// claimAdmits evaluates the configured claim rule. Matching is exact and
// case-sensitive, and an unexpected value shape is a non-match.
func claimAdmits(rule ClaimConfig, claims Claims) bool {
	value, ok := claimAt(rule.Path, claims)
	if !ok || len(rule.Values) == 0 {
		return false
	}
	present := claimStrings(value)
	if len(present) == 0 {
		return false
	}
	if rule.Match == MatchAll {
		for _, want := range rule.Values {
			if !containsString(present, want) {
				return false
			}
		}
		return true
	}
	for _, want := range rule.Values {
		if containsString(present, want) {
			return true
		}
	}
	return false
}

// claimAt resolves a JSON Pointer into the verified claim set.
func claimAt(pointer string, claims Claims) (json.RawMessage, bool) {
	if pointer == "" || !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	name := unescapePointerToken(tokens[0])
	current, ok := claims.raw[name]
	if !ok {
		return nil, false
	}
	for _, token := range tokens[1:] {
		var object map[string]json.RawMessage
		if json.Unmarshal(current, &object) != nil {
			return nil, false
		}
		current, ok = object[unescapePointerToken(token)]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func unescapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~1", "/")
	return strings.ReplaceAll(token, "~0", "~")
}

// claimStrings accepts one string or an array of strings and rejects every
// other shape.
func claimStrings(value json.RawMessage) []string {
	var single string
	if json.Unmarshal(value, &single) == nil {
		return []string{single}
	}
	var many []string
	if json.Unmarshal(value, &many) == nil {
		return many
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
