// Package passkey implements a bounded WebAuthn relying-party subset for
// passkey registration and authentication.
package passkey

import (
	"encoding/json"
	"errors"
	"io"
	"time"
)

var (
	ErrInvalidConfig = errors.New("passkey: invalid configuration")
	ErrInvalidState  = errors.New("passkey: invalid ceremony state")
	ErrExpired       = errors.New("passkey: ceremony expired")
	ErrMalformed     = errors.New("passkey: malformed response")
	ErrLimitExceeded = errors.New("passkey: input limit exceeded")
	ErrChallenge     = errors.New("passkey: challenge mismatch")
	ErrOrigin        = errors.New("passkey: origin mismatch")
	ErrRPID          = errors.New("passkey: RP ID mismatch")
	ErrUser          = errors.New("passkey: user mismatch")
	ErrCredential    = errors.New("passkey: credential mismatch")
	ErrFlags         = errors.New("passkey: authenticator flags rejected")
	ErrAlgorithm     = errors.New("passkey: unsupported algorithm")
	ErrSignature     = errors.New("passkey: invalid signature")
	ErrAttestation   = errors.New("passkey: unsupported attestation")
)

const ES256 = -7

type Config struct {
	RPID                  string
	RPName                string
	Origins               []string
	AllowLoopbackHTTP     bool
	Random                io.Reader
	Clock                 func() time.Time
	CeremonyTTL           time.Duration
	MaxJSONBytes          int
	MaxAttestationBytes   int
	MaxAuthenticatorBytes int
	MaxCredentialIDBytes  int
	MaxSignatureBytes     int
}

type RelyingParty struct {
	rpID                  string
	rpName                string
	origins               map[string]struct{}
	random                io.Reader
	now                   func() time.Time
	ceremonyTTL           time.Duration
	maxJSONBytes          int
	maxAttestationBytes   int
	maxAuthenticatorBytes int
	maxCredentialIDBytes  int
	maxSignatureBytes     int
}

func (rp *RelyingParty) ready() bool {
	return rp != nil && rp.rpID != "" && rp.rpName != "" && rp.origins != nil &&
		rp.random != nil && rp.now != nil && rp.ceremonyTTL > 0 && rp.maxJSONBytes > 0 &&
		rp.maxAttestationBytes > 0 && rp.maxAuthenticatorBytes > 0 &&
		rp.maxCredentialIDBytes > 0 && rp.maxSignatureBytes > 0
}

type User struct {
	ID          []byte
	Name        string
	DisplayName string
}

type RPEntity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type CredentialParameter struct {
	Type      string `json:"type"`
	Algorithm int    `json:"alg"`
}

type CredentialDescriptor struct {
	Type       string   `json:"type"`
	ID         string   `json:"id"`
	Transports []string `json:"transports,omitempty"`
}

type AuthenticatorSelection struct {
	ResidentKey      string `json:"residentKey,omitempty"`
	UserVerification string `json:"userVerification,omitempty"`
}

type CreationOptions struct {
	Challenge              string                 `json:"challenge"`
	RP                     RPEntity               `json:"rp"`
	User                   UserEntity             `json:"user"`
	PubKeyCredParams       []CredentialParameter  `json:"pubKeyCredParams"`
	Timeout                int                    `json:"timeout,omitempty"`
	ExcludeCredentials     []CredentialDescriptor `json:"excludeCredentials,omitempty"`
	AuthenticatorSelection AuthenticatorSelection `json:"authenticatorSelection,omitempty"`
	Attestation            string                 `json:"attestation"`
}

type RequestOptions struct {
	Challenge        string                 `json:"challenge"`
	Timeout          int                    `json:"timeout,omitempty"`
	RPID             string                 `json:"rpId"`
	AllowCredentials []CredentialDescriptor `json:"allowCredentials,omitempty"`
	UserVerification string                 `json:"userVerification,omitempty"`
}

type RegistrationOptions struct {
	Timeout                 time.Duration
	ExcludeCredentials      []CredentialDescriptor
	ResidentKey             string
	RequireUserVerification bool
}

type AuthenticationOptions struct {
	Timeout                 time.Duration
	AllowCredentials        []CredentialDescriptor
	RequireUserVerification bool
}

type ceremonyKind uint8

const (
	registrationCeremony ceremonyKind = iota + 1
	authenticationCeremony
)

// CeremonyState contains secret, expiring correlation data. Applications
// must store it without logging it and consume it exactly once.
type CeremonyState struct {
	kind                    ceremonyKind
	challenge               string
	expiresAt               time.Time
	userHandle              []byte
	allowedCredentialIDs    [][]byte
	requireUserVerification bool
}

func (s CeremonyState) ExpiresAt() time.Time { return s.expiresAt }

type RegistrationCredential struct {
	ID                     string                         `json:"id"`
	RawID                  string                         `json:"rawId"`
	Type                   string                         `json:"type"`
	Response               RegistrationCredentialResponse `json:"response"`
	ClientExtensionResults map[string]json.RawMessage     `json:"clientExtensionResults,omitempty"`
}

type RegistrationCredentialResponse struct {
	ClientDataJSON    string   `json:"clientDataJSON"`
	AttestationObject string   `json:"attestationObject"`
	Transports        []string `json:"transports,omitempty"`
}

type AuthenticationCredential struct {
	ID                     string                           `json:"id"`
	RawID                  string                           `json:"rawId"`
	Type                   string                           `json:"type"`
	Response               AuthenticationCredentialResponse `json:"response"`
	ClientExtensionResults map[string]json.RawMessage       `json:"clientExtensionResults,omitempty"`
}

type AuthenticationCredentialResponse struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AuthenticatorData string `json:"authenticatorData"`
	Signature         string `json:"signature"`
	UserHandle        string `json:"userHandle,omitempty"`
}

type CredentialRecord struct {
	ID             []byte
	UserHandle     []byte
	PublicKeyCOSE  []byte
	PublicKeyX     []byte
	PublicKeyY     []byte
	Algorithm      int
	SignCount      uint32
	BackupEligible bool
	BackupState    bool
	Transports     []string
	AAGUID         [16]byte
}

type RegistrationResult struct {
	Credential  CredentialRecord
	Attestation string
}

type AuthenticationResult struct {
	SignCount   uint32
	BackupState bool
	CounterRisk bool
}
