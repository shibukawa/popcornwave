// Package passkeytest is a test-only software authenticator for
// [github.com/shibukawa/popcornweb/contrib/passkey].
//
// It answers a ceremony with the same JSON a browser posts, so a passkey flow
// runs without a browser, hardware, or a human gesture. It reads the challenge,
// the RP ID, and the credential descriptors from the options the relying party
// emitted, and never from the ceremony state, so a broken wire format fails the
// test instead of passing it.
//
// The package performs no relying-party check and grants no session. It must
// never reach an application binary: it holds a signing key that mints
// assertions a relying party accepts.
package passkeytest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/shibukawa/popcornweb/contrib/passkey"
)

var (
	// ErrNoCredential means no stored credential satisfies the request.
	ErrNoCredential = errors.New("passkeytest: no credential for this request")
	// ErrExcluded means the relying party listed a credential this
	// authenticator already holds, so a real authenticator would refuse.
	ErrExcluded = errors.New("passkeytest: credential already registered")
	// ErrOptions means the emitted options are unusable.
	ErrOptions = errors.New("passkeytest: unusable ceremony options")
)

// Fault names one invalid response this authenticator can produce. A fault
// travels through the same code path as a valid ceremony, so a negative test
// exercises the real encoder rather than a hand-written blob.
type Fault string

const (
	FaultNone Fault = ""
	// FaultOrigin signs a clientDataJSON naming another origin.
	FaultOrigin Fault = "origin"
	// FaultRPID hashes another RP ID into the authenticator data.
	FaultRPID Fault = "rp_id"
	// FaultChallenge echoes a challenge the relying party did not send.
	FaultChallenge Fault = "challenge"
	// FaultUserPresence clears UP, which every ceremony must reject.
	FaultUserPresence Fault = "user_presence"
	// FaultUserVerification clears UV while the relying party required it.
	FaultUserVerification Fault = "user_verification"
	// FaultBackupState sets BS without BE, which is an invalid combination.
	FaultBackupState Fault = "backup_state"
	// FaultSignCount replays the previous counter instead of increasing it.
	FaultSignCount Fault = "sign_count"
	// FaultSignature corrupts the assertion signature.
	FaultSignature Fault = "signature"
	// FaultAlgorithm registers a COSE algorithm the relying party rejects.
	FaultAlgorithm Fault = "algorithm"
	// FaultUserHandle returns a user handle bound to nobody.
	FaultUserHandle Fault = "user_handle"
)

// Credential is the public view of one stored credential.
type Credential struct {
	ID         []byte
	RPID       string
	UserHandle []byte
	SignCount  uint32
}

type storedCredential struct {
	id           []byte
	rpID         string
	userHandle   []byte
	key          *ecdsa.PrivateKey
	signCount    uint32
	discoverable bool
}

// Authenticator is one software authenticator. It models a single device: it
// holds every credential registered through it, and two instances are
// independent, so a test can model two devices or two users.
//
// An Authenticator is not safe for concurrent use.
type Authenticator struct {
	origin         string
	algorithm      int
	aaguid         [16]byte
	transports     []string
	discoverable   bool
	userVerified   bool
	backupEligible bool
	backupState    bool
	random         io.Reader
	fault          Fault
	credentials    []*storedCredential
}

// Option configures an Authenticator.
type Option func(*Authenticator) error

// WithOrigin sets the origin this authenticator claims in clientDataJSON. It
// must match an origin the relying party allows.
func WithOrigin(origin string) Option {
	return func(a *Authenticator) error {
		if origin == "" {
			return fmt.Errorf("%w: empty origin", ErrOptions)
		}
		a.origin = origin
		return nil
	}
}

// WithAlgorithm selects the COSE algorithm of newly created credentials.
func WithAlgorithm(algorithm int) Option {
	return func(a *Authenticator) error {
		a.algorithm = algorithm
		return nil
	}
}

// WithSeed makes key material and credential IDs reproducible, so a failing run
// can be replayed. Signatures still vary, because ECDSA is randomized exactly
// as it is on a real authenticator.
func WithSeed(seed string) Option {
	return func(a *Authenticator) error {
		if seed == "" {
			return fmt.Errorf("%w: empty seed", ErrOptions)
		}
		a.random = newSeededReader(seed)
		return nil
	}
}

// WithDiscoverable controls whether credentials are discoverable, which decides
// whether an assertion returns a user handle.
func WithDiscoverable(discoverable bool) Option {
	return func(a *Authenticator) error {
		a.discoverable = discoverable
		return nil
	}
}

// WithTransports sets the transport hints a registration reports.
func WithTransports(transports ...string) Option {
	return func(a *Authenticator) error {
		a.transports = append([]string(nil), transports...)
		return nil
	}
}

// WithAAGUID sets the authenticator model identifier.
func WithAAGUID(aaguid [16]byte) Option {
	return func(a *Authenticator) error {
		a.aaguid = aaguid
		return nil
	}
}

// WithUserVerification controls the UV flag, which models whether the device
// verified the user rather than merely observing presence.
func WithUserVerification(verified bool) Option {
	return func(a *Authenticator) error {
		a.userVerified = verified
		return nil
	}
}

// WithBackup sets backup eligibility and current backup state, which a relying
// party persists and compares across ceremonies.
func WithBackup(eligible, state bool) Option {
	return func(a *Authenticator) error {
		a.backupEligible = eligible
		a.backupState = state
		return nil
	}
}

// WithFault makes every subsequent ceremony produce one specific invalid
// response. Use [Authenticator.SetFault] to scope it to one call.
func WithFault(fault Fault) Option {
	return func(a *Authenticator) error {
		a.fault = fault
		return nil
	}
}

// NewAuthenticator returns an authenticator that holds no credential yet. The
// default is a discoverable, user-verified, backup-eligible internal
// authenticator on https://example.com.
func NewAuthenticator(options ...Option) (*Authenticator, error) {
	authenticator := &Authenticator{
		origin:         "https://example.com",
		algorithm:      passkey.ES256,
		transports:     []string{"internal"},
		discoverable:   true,
		userVerified:   true,
		backupEligible: true,
		random:         rand.Reader,
	}
	for _, apply := range options {
		if apply == nil {
			continue
		}
		if err := apply(authenticator); err != nil {
			return nil, err
		}
	}
	return authenticator, nil
}

// SetFault changes the fault of an existing authenticator, so one test can
// alternate between valid and invalid ceremonies.
func (a *Authenticator) SetFault(fault Fault) { a.fault = fault }

// Origin returns the origin this authenticator claims.
func (a *Authenticator) Origin() string { return a.origin }

// Credentials returns the stored credentials in registration order.
func (a *Authenticator) Credentials() []Credential {
	result := make([]Credential, 0, len(a.credentials))
	for _, stored := range a.credentials {
		result = append(result, Credential{
			ID:         append([]byte(nil), stored.id...),
			RPID:       stored.rpID,
			UserHandle: append([]byte(nil), stored.userHandle...),
			SignCount:  stored.signCount,
		})
	}
	return result
}

// Create answers a registration ceremony. It reads the challenge, the RP ID,
// the user handle, and excludeCredentials from options alone.
func (a *Authenticator) Create(options passkey.CreationOptions) (passkey.RegistrationCredential, error) {
	var zero passkey.RegistrationCredential
	if options.RP.ID == "" {
		return zero, fmt.Errorf("%w: empty rp.id", ErrOptions)
	}
	if options.Challenge == "" {
		return zero, fmt.Errorf("%w: empty challenge", ErrOptions)
	}
	userHandle, err := base64.RawURLEncoding.DecodeString(options.User.ID)
	if err != nil || len(userHandle) == 0 {
		return zero, fmt.Errorf("%w: unusable user.id", ErrOptions)
	}
	for _, descriptor := range options.ExcludeCredentials {
		id, err := base64.RawURLEncoding.DecodeString(descriptor.ID)
		if err != nil {
			return zero, fmt.Errorf("%w: unusable excludeCredentials entry", ErrOptions)
		}
		if a.find(options.RP.ID, id) != nil {
			return zero, ErrExcluded
		}
	}

	key, credentialID, err := a.newCredential()
	if err != nil {
		return zero, err
	}
	algorithm := a.algorithm
	if a.fault == FaultAlgorithm {
		// RS256, which the relying party does not implement.
		algorithm = -257
	}
	cose, err := encodeCOSEKey(&key.PublicKey, algorithm)
	if err != nil {
		return zero, err
	}
	clientData, err := a.clientData("webauthn.create", options.Challenge)
	if err != nil {
		return zero, err
	}
	authData := registrationAuthData(a.hashedRPID(options.RP.ID), a.flags(flagAT), 0, a.aaguid, credentialID, cose)
	attestation, err := encodeAttestationObject(authData)
	if err != nil {
		return zero, err
	}

	a.credentials = append(a.credentials, &storedCredential{
		id: credentialID, rpID: options.RP.ID, userHandle: userHandle,
		key: key, discoverable: a.discoverable,
	})
	encodedID := base64.RawURLEncoding.EncodeToString(credentialID)
	return passkey.RegistrationCredential{
		ID: encodedID, RawID: encodedID, Type: "public-key",
		Response: passkey.RegistrationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AttestationObject: base64.RawURLEncoding.EncodeToString(attestation),
			Transports:        append([]string(nil), a.transports...),
		},
	}, nil
}

// Get answers an authentication ceremony. It selects a credential from
// allowCredentials, or the first discoverable credential of the RP ID when the
// relying party sent none.
func (a *Authenticator) Get(options passkey.RequestOptions) (passkey.AuthenticationCredential, error) {
	var zero passkey.AuthenticationCredential
	if options.RPID == "" {
		return zero, fmt.Errorf("%w: empty rpId", ErrOptions)
	}
	if options.Challenge == "" {
		return zero, fmt.Errorf("%w: empty challenge", ErrOptions)
	}
	selected, err := a.selectCredential(options)
	if err != nil {
		return zero, err
	}

	signCount := selected.signCount + 1
	if a.fault == FaultSignCount {
		// A cloned authenticator replays a counter it already used.
		signCount = selected.signCount
	}
	clientData, err := a.clientData("webauthn.get", options.Challenge)
	if err != nil {
		return zero, err
	}
	authData := assertionAuthData(a.hashedRPID(options.RPID), a.flags(0), signCount)
	clientHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte(nil), authData...), clientHash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, selected.key, digest[:])
	if err != nil {
		return zero, fmt.Errorf("passkeytest: sign: %w", err)
	}
	if a.fault == FaultSignature {
		signature = append([]byte(nil), signature...)
		signature[len(signature)-1] ^= 0xff
	}
	selected.signCount = signCount

	encodedID := base64.RawURLEncoding.EncodeToString(selected.id)
	response := passkey.AuthenticationCredential{
		ID: encodedID, RawID: encodedID, Type: "public-key",
		Response: passkey.AuthenticationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
			Signature:         base64.RawURLEncoding.EncodeToString(signature),
		},
	}
	switch {
	case a.fault == FaultUserHandle:
		response.Response.UserHandle = base64.RawURLEncoding.EncodeToString([]byte("not-this-account"))
	case selected.discoverable:
		response.Response.UserHandle = base64.RawURLEncoding.EncodeToString(selected.userHandle)
	}
	return response, nil
}

func (a *Authenticator) selectCredential(options passkey.RequestOptions) (*storedCredential, error) {
	if len(options.AllowCredentials) > 0 {
		for _, descriptor := range options.AllowCredentials {
			id, err := base64.RawURLEncoding.DecodeString(descriptor.ID)
			if err != nil {
				return nil, fmt.Errorf("%w: unusable allowCredentials entry", ErrOptions)
			}
			if found := a.find(options.RPID, id); found != nil {
				return found, nil
			}
		}
		return nil, ErrNoCredential
	}
	for _, stored := range a.credentials {
		if stored.rpID == options.RPID && stored.discoverable {
			return stored, nil
		}
	}
	return nil, ErrNoCredential
}

func (a *Authenticator) find(rpID string, id []byte) *storedCredential {
	for _, stored := range a.credentials {
		if stored.rpID == rpID && len(stored.id) == len(id) && string(stored.id) == string(id) {
			return stored
		}
	}
	return nil
}

// flags builds the authenticator data flag byte, applying the flag faults.
func (a *Authenticator) flags(extra byte) byte {
	flags := flagUP | extra
	if a.userVerified {
		flags |= flagUV
	}
	if a.backupEligible {
		flags |= flagBE
	}
	if a.backupState {
		flags |= flagBS
	}
	switch a.fault {
	case FaultUserPresence:
		flags &^= flagUP
	case FaultUserVerification:
		flags &^= flagUV
	case FaultBackupState:
		// Backed up without being eligible for backup, which cannot happen.
		flags |= flagBS
		flags &^= flagBE
	}
	return flags
}

// hashedRPID returns the RP ID that goes into the authenticator data hash.
func (a *Authenticator) hashedRPID(rpID string) string {
	if a.fault == FaultRPID {
		return "attacker." + rpID
	}
	return rpID
}

type clientDataJSON struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func (a *Authenticator) clientData(ceremony, challenge string) ([]byte, error) {
	data := clientDataJSON{Type: ceremony, Challenge: challenge, Origin: a.origin}
	switch a.fault {
	case FaultOrigin:
		data.Origin = "https://attacker.example"
	case FaultChallenge:
		data.Challenge = base64.RawURLEncoding.EncodeToString([]byte("challenge-the-server-never-sent"))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("passkeytest: client data: %w", err)
	}
	return encoded, nil
}

// newCredential derives a P-256 key and a credential ID from the configured
// reader. The key is built from a scalar rather than through ecdsa.GenerateKey,
// because that function mixes in global randomness and would defeat WithSeed.
func (a *Authenticator) newCredential() (*ecdsa.PrivateKey, []byte, error) {
	credentialID := make([]byte, 32)
	if _, err := io.ReadFull(a.random, credentialID); err != nil {
		return nil, nil, fmt.Errorf("passkeytest: credential id: %w", err)
	}
	scalar := make([]byte, 32)
	if _, err := io.ReadFull(a.random, scalar); err != nil {
		return nil, nil, fmt.Errorf("passkeytest: key material: %w", err)
	}
	curve := elliptic.P256()
	order := curve.Params().N
	d := new(big.Int).SetBytes(scalar)
	// Reduce into [1, N-1]; a zero scalar has no public key.
	d.Mod(d, new(big.Int).Sub(order, big.NewInt(1)))
	d.Add(d, big.NewInt(1))
	key := &ecdsa.PrivateKey{D: d}
	key.PublicKey.Curve = curve
	key.PublicKey.X, key.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	return key, credentialID, nil
}

// seededReader is a deterministic byte stream derived from a seed. It exists so
// a failing run can be replayed, and is never used for anything a real
// deployment depends on.
type seededReader struct {
	state   [sha256.Size]byte
	pending []byte
}

func newSeededReader(seed string) *seededReader {
	return &seededReader{state: sha256.Sum256([]byte("passkeytest/" + seed))}
}

func (r *seededReader) Read(p []byte) (int, error) {
	for len(r.pending) < len(p) {
		r.state = sha256.Sum256(r.state[:])
		r.pending = append(r.pending, r.state[:]...)
	}
	copied := copy(p, r.pending)
	r.pending = r.pending[copied:]
	return copied, nil
}
