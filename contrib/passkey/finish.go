package passkey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/subtle"
	"math/big"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
)

func (rp *RelyingParty) FinishRegistration(state CeremonyState, response RegistrationCredential) (RegistrationResult, error) {
	if !rp.ready() {
		return RegistrationResult{}, ErrInvalidConfig
	}
	if err := rp.validateState(state, registrationCeremony); err != nil {
		return RegistrationResult{}, err
	}
	credentialID, err := rp.decodeCredential(response.ID, response.RawID, response.Type)
	if err != nil {
		return RegistrationResult{}, err
	}
	for _, excluded := range state.allowedCredentialIDs {
		if equalBytes(credentialID, excluded) {
			return RegistrationResult{}, ErrCredential
		}
	}
	if _, _, err := rp.parseClientData(response.Response.ClientDataJSON, "webauthn.create", state); err != nil {
		return RegistrationResult{}, err
	}
	authenticatorRaw, err := rp.parseAttestationObject(response.Response.AttestationObject)
	if err != nil {
		return RegistrationResult{}, err
	}
	authenticator, err := rp.parseAuthenticatorData(authenticatorRaw, true)
	if err != nil {
		return RegistrationResult{}, err
	}
	if state.requireUserVerification && authenticator.flags&flagUV == 0 {
		return RegistrationResult{}, ErrFlags
	}
	if !equalBytes(credentialID, authenticator.credentialID) {
		return RegistrationResult{}, ErrCredential
	}
	transports, err := validateTransports(response.Response.Transports)
	if err != nil {
		return RegistrationResult{}, err
	}
	return RegistrationResult{
		Credential: CredentialRecord{
			ID: append([]byte(nil), credentialID...), UserHandle: append([]byte(nil), state.userHandle...),
			PublicKeyCOSE: append([]byte(nil), authenticator.credentialCOSE...),
			PublicKeyX:    append([]byte(nil), authenticator.publicKeyX...), PublicKeyY: append([]byte(nil), authenticator.publicKeyY...),
			Algorithm: authenticator.algorithm, SignCount: authenticator.signCount,
			BackupEligible: authenticator.flags&flagBE != 0, BackupState: authenticator.flags&flagBS != 0,
			Transports: transports, AAGUID: authenticator.aaguid,
		},
		Attestation: "none",
	}, nil
}

func (rp *RelyingParty) FinishAuthentication(state CeremonyState, response AuthenticationCredential, credential CredentialRecord) (AuthenticationResult, error) {
	if !rp.ready() {
		return AuthenticationResult{}, ErrInvalidConfig
	}
	if err := rp.validateState(state, authenticationCeremony); err != nil {
		return AuthenticationResult{}, err
	}
	credentialID, err := rp.decodeCredential(response.ID, response.RawID, response.Type)
	if err != nil {
		return AuthenticationResult{}, err
	}
	if !equalBytes(credentialID, credential.ID) || !allowedCredential(credentialID, state.allowedCredentialIDs) {
		return AuthenticationResult{}, ErrCredential
	}
	if len(state.userHandle) != 0 && !equalBytes(state.userHandle, credential.UserHandle) {
		return AuthenticationResult{}, ErrUser
	}
	userHandle, err := rp.decodeUserHandle(response.Response.UserHandle)
	if err != nil {
		return AuthenticationResult{}, err
	}
	if len(state.userHandle) == 0 && len(userHandle) == 0 {
		return AuthenticationResult{}, ErrUser
	}
	if len(userHandle) != 0 && !equalBytes(userHandle, credential.UserHandle) {
		return AuthenticationResult{}, ErrUser
	}
	clientDataJSON, _, err := rp.parseClientData(response.Response.ClientDataJSON, "webauthn.get", state)
	if err != nil {
		return AuthenticationResult{}, err
	}
	authenticatorRaw, err := authn.DecodeBase64URL(
		response.Response.AuthenticatorData, encodedLimit(rp.maxAuthenticatorBytes), rp.maxAuthenticatorBytes,
	)
	if err != nil {
		return AuthenticationResult{}, classifyInputError(err)
	}
	authenticator, err := rp.parseAuthenticatorData(authenticatorRaw, false)
	if err != nil {
		return AuthenticationResult{}, err
	}
	if state.requireUserVerification && authenticator.flags&flagUV == 0 {
		return AuthenticationResult{}, ErrFlags
	}
	if (authenticator.flags&flagBE != 0) != credential.BackupEligible {
		return AuthenticationResult{}, ErrFlags
	}
	publicKey, err := credentialES256Key(credential)
	if err != nil {
		return AuthenticationResult{}, err
	}
	signature, err := authn.DecodeBase64URL(
		response.Response.Signature, encodedLimit(rp.maxSignatureBytes), rp.maxSignatureBytes,
	)
	if err != nil || len(signature) == 0 {
		return AuthenticationResult{}, ErrSignature
	}
	clientHash := sha256.Sum256(clientDataJSON)
	signed := make([]byte, 0, len(authenticatorRaw)+len(clientHash))
	signed = append(signed, authenticatorRaw...)
	signed = append(signed, clientHash[:]...)
	digest := sha256.Sum256(signed)
	if !ecdsa.VerifyASN1(publicKey, digest[:], signature) {
		return AuthenticationResult{}, ErrSignature
	}
	counterRisk := false
	if authenticator.signCount != 0 || credential.SignCount != 0 {
		counterRisk = authenticator.signCount <= credential.SignCount
	}
	return AuthenticationResult{
		SignCount: authenticator.signCount, BackupState: authenticator.flags&flagBS != 0, CounterRisk: counterRisk,
	}, nil
}

func (rp *RelyingParty) validateState(state CeremonyState, expected ceremonyKind) error {
	if state.kind != expected || state.challenge == "" || len(state.userHandle) > maxUserHandleBytes {
		return ErrInvalidState
	}
	if err := authn.RequireUnexpired(rp.now(), state.expiresAt); err != nil {
		return ErrExpired
	}
	return nil
}

func (rp *RelyingParty) decodeUserHandle(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	handle, err := authn.DecodeBase64URL(encoded, encodedLimit(maxUserHandleBytes), maxUserHandleBytes)
	if err != nil || len(handle) == 0 {
		return nil, ErrUser
	}
	return handle, nil
}

func credentialES256Key(credential CredentialRecord) (*ecdsa.PublicKey, error) {
	if credential.Algorithm != ES256 || len(credential.PublicKeyX) != 32 || len(credential.PublicKeyY) != 32 {
		return nil, ErrAlgorithm
	}
	if len(credential.PublicKeyCOSE) != 0 {
		x, y, algorithm, err := parseCOSEKey(credential.PublicKeyCOSE)
		if err != nil || algorithm != credential.Algorithm || !equalBytes(x, credential.PublicKeyX) || !equalBytes(y, credential.PublicKeyY) {
			return nil, ErrAlgorithm
		}
	}
	x := new(big.Int).SetBytes(credential.PublicKeyX)
	y := new(big.Int).SetBytes(credential.PublicKeyY)
	if !elliptic.P256().IsOnCurve(x, y) {
		return nil, ErrAlgorithm
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare(left, right) == 1
}

func allowedCredential(id []byte, allowed [][]byte) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if equalBytes(id, candidate) {
			return true
		}
	}
	return false
}

func validateTransports(transports []string) ([]string, error) {
	if len(transports) > 8 {
		return nil, ErrLimitExceeded
	}
	result := make([]string, 0, len(transports))
	seen := make(map[string]struct{}, len(transports))
	for _, transport := range transports {
		switch transport {
		case "usb", "nfc", "ble", "smart-card", "hybrid", "internal":
		default:
			return nil, ErrMalformed
		}
		if _, duplicate := seen[transport]; duplicate {
			continue
		}
		seen[transport] = struct{}{}
		result = append(result, transport)
	}
	return result, nil
}
