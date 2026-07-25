package passkey

import (
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
)

const (
	defaultCeremonyTTL           = 5 * time.Minute
	defaultMaxJSONBytes          = 64 << 10
	defaultMaxAttestationBytes   = 128 << 10
	defaultMaxAuthenticatorBytes = 128 << 10
	defaultMaxCredentialIDBytes  = 1023
	defaultMaxSignatureBytes     = 512
	maximumMaxJSONBytes          = 1 << 20
	maximumMaxAttestationBytes   = 4 << 20
	maximumMaxAuthenticatorBytes = 1 << 20
	maximumMaxSignatureBytes     = 4 << 10
	maxUserHandleBytes           = 64
	maxCeremonyTTL               = 30 * time.Minute
	maxRPNameBytes               = 256
	maxUserNameBytes             = 256
	maxOriginBytes               = 2048
	maxCredentialDescriptors     = 64
	maxTransports                = 8
	maxTransportBytes            = 32
)

func New(config Config) (*RelyingParty, error) {
	if !validRPID(config.RPID) ||
		config.RPName == "" || len(config.RPName) > maxRPNameBytes || len(config.Origins) == 0 || len(config.Origins) > maxCredentialDescriptors ||
		config.CeremonyTTL < 0 || config.CeremonyTTL > maxCeremonyTTL ||
		config.MaxJSONBytes < 0 || config.MaxAttestationBytes < 0 ||
		config.MaxAuthenticatorBytes < 0 || config.MaxCredentialIDBytes < 0 || config.MaxSignatureBytes < 0 ||
		config.MaxJSONBytes > maximumMaxJSONBytes || config.MaxAttestationBytes > maximumMaxAttestationBytes ||
		config.MaxAuthenticatorBytes > maximumMaxAuthenticatorBytes || config.MaxCredentialIDBytes > defaultMaxCredentialIDBytes ||
		config.MaxSignatureBytes > maximumMaxSignatureBytes {
		return nil, ErrInvalidConfig
	}
	origins := make(map[string]struct{}, len(config.Origins))
	for _, origin := range config.Origins {
		if len(origin) > maxOriginBytes {
			return nil, ErrInvalidConfig
		}
		if !validOrigin(origin, config.AllowLoopbackHTTP) || !originMatchesRPID(origin, config.RPID) {
			return nil, ErrInvalidConfig
		}
		origins[origin] = struct{}{}
	}
	random := config.Random
	if random == nil {
		random = rand.Reader
	}
	now := config.Clock
	if now == nil {
		now = time.Now
	}
	return &RelyingParty{
		rpID: config.RPID, rpName: config.RPName, origins: origins, random: random, now: now,
		ceremonyTTL:           defaultedDuration(config.CeremonyTTL, defaultCeremonyTTL),
		maxJSONBytes:          defaultedInt(config.MaxJSONBytes, defaultMaxJSONBytes),
		maxAttestationBytes:   defaultedInt(config.MaxAttestationBytes, defaultMaxAttestationBytes),
		maxAuthenticatorBytes: defaultedInt(config.MaxAuthenticatorBytes, defaultMaxAuthenticatorBytes),
		maxCredentialIDBytes:  defaultedInt(config.MaxCredentialIDBytes, defaultMaxCredentialIDBytes),
		maxSignatureBytes:     defaultedInt(config.MaxSignatureBytes, defaultMaxSignatureBytes),
	}, nil
}

func validRPID(value string) bool {
	if value == "" || len(value) > 253 || strings.ContainsAny(value, "/:@[]\\") {
		return false
	}
	value = strings.TrimSuffix(value, ".")
	if value == "" {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') || char == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func (rp *RelyingParty) BeginRegistration(user User, options RegistrationOptions) (CreationOptions, CeremonyState, error) {
	if !rp.ready() {
		return CreationOptions{}, CeremonyState{}, ErrInvalidConfig
	}
	if len(user.ID) == 0 || len(user.ID) > maxUserHandleBytes || user.Name == "" || len(user.Name) > maxUserNameBytes || user.DisplayName == "" || len(user.DisplayName) > maxUserNameBytes {
		return CreationOptions{}, CeremonyState{}, ErrUser
	}
	excluded, err := rp.validateDescriptors(options.ExcludeCredentials)
	if err != nil {
		return CreationOptions{}, CeremonyState{}, err
	}
	challenge, err := authn.GenerateSecret(rp.random, 32)
	if err != nil {
		return CreationOptions{}, CeremonyState{}, err
	}
	verification := "preferred"
	if options.RequireUserVerification {
		verification = "required"
	}
	residentKey := options.ResidentKey
	if residentKey == "" {
		residentKey = "preferred"
	}
	if residentKey != "discouraged" && residentKey != "preferred" && residentKey != "required" {
		return CreationOptions{}, CeremonyState{}, ErrInvalidConfig
	}
	state := CeremonyState{
		kind: registrationCeremony, challenge: challenge, expiresAt: rp.now().Add(rp.ceremonyTTL),
		userHandle: append([]byte(nil), user.ID...), allowedCredentialIDs: excluded,
		requireUserVerification: options.RequireUserVerification,
	}
	return CreationOptions{
		Challenge: challenge,
		RP:        RPEntity{ID: rp.rpID, Name: rp.rpName},
		User: UserEntity{
			ID: base64.RawURLEncoding.EncodeToString(user.ID), Name: user.Name, DisplayName: user.DisplayName,
		},
		PubKeyCredParams:   []CredentialParameter{{Type: "public-key", Algorithm: ES256}},
		Timeout:            timeoutMilliseconds(options.Timeout),
		ExcludeCredentials: append([]CredentialDescriptor(nil), options.ExcludeCredentials...),
		AuthenticatorSelection: AuthenticatorSelection{
			ResidentKey: residentKey, UserVerification: verification,
		},
		Attestation: "none",
	}, state, nil
}

// BeginAuthentication starts account-bound authentication when userHandle is
// non-empty and discoverable authentication when it is empty.
func (rp *RelyingParty) BeginAuthentication(userHandle []byte, options AuthenticationOptions) (RequestOptions, CeremonyState, error) {
	if !rp.ready() {
		return RequestOptions{}, CeremonyState{}, ErrInvalidConfig
	}
	if len(userHandle) > maxUserHandleBytes {
		return RequestOptions{}, CeremonyState{}, ErrUser
	}
	allowed, err := rp.validateDescriptors(options.AllowCredentials)
	if err != nil {
		return RequestOptions{}, CeremonyState{}, err
	}
	challenge, err := authn.GenerateSecret(rp.random, 32)
	if err != nil {
		return RequestOptions{}, CeremonyState{}, err
	}
	verification := "preferred"
	if options.RequireUserVerification {
		verification = "required"
	}
	state := CeremonyState{
		kind: authenticationCeremony, challenge: challenge, expiresAt: rp.now().Add(rp.ceremonyTTL),
		userHandle: append([]byte(nil), userHandle...), allowedCredentialIDs: allowed,
		requireUserVerification: options.RequireUserVerification,
	}
	return RequestOptions{
		Challenge: challenge, Timeout: timeoutMilliseconds(options.Timeout), RPID: rp.rpID,
		AllowCredentials: append([]CredentialDescriptor(nil), options.AllowCredentials...),
		UserVerification: verification,
	}, state, nil
}

func (rp *RelyingParty) validateDescriptors(descriptors []CredentialDescriptor) ([][]byte, error) {
	if len(descriptors) > maxCredentialDescriptors {
		return nil, ErrLimitExceeded
	}
	result := make([][]byte, 0, len(descriptors))
	seen := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.Type != "public-key" {
			return nil, ErrCredential
		}
		if len(descriptor.Transports) > maxTransports {
			return nil, ErrLimitExceeded
		}
		for _, transport := range descriptor.Transports {
			if transport == "" || len(transport) > maxTransportBytes {
				return nil, ErrCredential
			}
		}
		id, err := authn.DecodeBase64URL(descriptor.ID, encodedLimit(rp.maxCredentialIDBytes), rp.maxCredentialIDBytes)
		if err != nil || len(id) == 0 {
			return nil, ErrCredential
		}
		key := string(id)
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrCredential
		}
		seen[key] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func validOrigin(raw string, allowLoopbackHTTP bool) bool {
	origin, err := url.Parse(raw)
	if err != nil || origin.Host == "" || origin.Hostname() == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || origin.Path != "" {
		return false
	}
	if origin.Scheme == "https" {
		return true
	}
	if origin.Scheme != "http" || !allowLoopbackHTTP {
		return false
	}
	host := origin.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func originMatchesRPID(rawOrigin, rpID string) bool {
	origin, err := url.Parse(rawOrigin)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(origin.Hostname()), ".")
	rpID = strings.TrimSuffix(strings.ToLower(rpID), ".")
	if host == rpID {
		return true
	}
	if net.ParseIP(host) != nil || net.ParseIP(rpID) != nil {
		return false
	}
	return strings.HasSuffix(host, "."+rpID)
}

func defaultedInt(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

func defaultedDuration(value, defaultValue time.Duration) time.Duration {
	if value == 0 {
		return defaultValue
	}
	return value
}

func timeoutMilliseconds(timeout time.Duration) int {
	if timeout <= 0 {
		return 0
	}
	maximum := 10 * time.Minute
	if timeout > maximum {
		timeout = maximum
	}
	return int(timeout / time.Millisecond)
}

func encodedLimit(decoded int) int {
	return (decoded*4 + 2) / 3
}
