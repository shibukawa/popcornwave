package passkey

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/contrib/authstate/memory"
	"github.com/shibukawa/popcornwave/contrib/cbor"
)

type passkeyFixture struct {
	rp           *RelyingParty
	now          time.Time
	user         User
	privateKey   *ecdsa.PrivateKey
	credentialID []byte
	registration RegistrationResult
}

func TestNilRelyingPartyIsSafe(t *testing.T) {
	var rp *RelyingParty
	if _, _, err := rp.BeginRegistration(User{ID: []byte("u"), Name: "u", DisplayName: "U"}, RegistrationOptions{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("BeginRegistration = %v", err)
	}
	if _, _, err := rp.BeginAuthentication(nil, AuthenticationOptions{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("BeginAuthentication = %v", err)
	}
	if _, err := rp.DecodeRegistrationCredential([]byte(`{}`)); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("DecodeRegistrationCredential = %v", err)
	}
	if _, err := rp.DecodeAuthenticationCredential([]byte(`{}`)); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("DecodeAuthenticationCredential = %v", err)
	}
	if _, err := rp.FinishRegistration(CeremonyState{}, RegistrationCredential{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("FinishRegistration = %v", err)
	}
	if _, err := rp.FinishAuthentication(CeremonyState{}, AuthenticationCredential{}, CredentialRecord{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("FinishAuthentication = %v", err)
	}
}

func TestZeroRelyingPartyIsSafe(t *testing.T) {
	var rp RelyingParty
	if _, _, err := rp.BeginRegistration(User{ID: []byte("u"), Name: "u", DisplayName: "U"}, RegistrationOptions{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero BeginRegistration = %v", err)
	}
	if _, err := rp.DecodeAuthenticationCredential([]byte(`{}`)); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero DecodeAuthenticationCredential = %v", err)
	}
}

func TestRejectsMalformedRPID(t *testing.T) {
	for _, rpID := range []string{"example..com", "-example.com", "example.com-", "example/com", "https://example.com"} {
		if _, err := New(Config{RPID: rpID, RPName: "Example", Origins: []string{"https://example.com"}}); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("RPID %q error = %v", rpID, err)
		}
	}
}

func TestRejectsUnboundedGenerationInputs(t *testing.T) {
	if _, err := New(Config{RPID: "example.com", RPName: strings.Repeat("r", maxRPNameBytes+1), Origins: []string{"https://example.com"}}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("oversized RPName error = %v", err)
	}
	rp := newFixture(t).rp
	if _, _, err := rp.BeginRegistration(User{ID: []byte("u"), Name: strings.Repeat("n", maxUserNameBytes+1), DisplayName: "User"}, RegistrationOptions{}); !errors.Is(err, ErrUser) {
		t.Fatalf("oversized user name error = %v", err)
	}
	descriptors := make([]CredentialDescriptor, maxCredentialDescriptors+1)
	for i := range descriptors {
		descriptors[i] = CredentialDescriptor{Type: "public-key", ID: base64.RawURLEncoding.EncodeToString([]byte{byte(i + 1)})}
	}
	if _, _, err := rp.BeginAuthentication(nil, AuthenticationOptions{AllowCredentials: descriptors}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("oversized descriptor list error = %v", err)
	}
	transports := make([]string, maxTransports+1)
	for i := range transports {
		transports[i] = "internal"
	}
	descriptor := CredentialDescriptor{Type: "public-key", ID: base64.RawURLEncoding.EncodeToString([]byte("id")), Transports: transports}
	if _, _, err := rp.BeginAuthentication(nil, AuthenticationOptions{AllowCredentials: []CredentialDescriptor{descriptor}}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("oversized transport list error = %v", err)
	}
}

func newFixture(t *testing.T) *passkeyFixture {
	t.Helper()
	now := time.Unix(1_700_000_000, 0)
	rp, err := New(Config{
		RPID: "example.com", RPName: "Example", Origins: []string{"https://example.com"},
		Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &passkeyFixture{
		rp: rp, now: now, user: User{ID: []byte("user-1"), Name: "user", DisplayName: "User"},
		privateKey: privateKey, credentialID: bytes.Repeat([]byte{0x42}, 32),
	}
}

func (fixture *passkeyFixture) register(t *testing.T, flags byte, algorithm int) RegistrationResult {
	t.Helper()
	creation, state, err := fixture.rp.BeginRegistration(fixture.user, RegistrationOptions{RequireUserVerification: true})
	if err != nil {
		t.Fatal(err)
	}
	clientData := fixture.clientData(t, "webauthn.create", creation.Challenge, "https://example.com")
	cose := encodeCOSEKey(t, &fixture.privateKey.PublicKey, algorithm)
	authData := registrationAuthData("example.com", flags, 0, fixture.credentialID, cose)
	encodedID := base64.RawURLEncoding.EncodeToString(fixture.credentialID)
	result, err := fixture.rp.FinishRegistration(state, RegistrationCredential{
		ID: encodedID, RawID: encodedID, Type: "public-key",
		Response: RegistrationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AttestationObject: base64.RawURLEncoding.EncodeToString(encodeAttestationObject(t, authData)),
			Transports:        []string{"internal"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.registration = result
	return result
}

func (fixture *passkeyFixture) authenticationResponse(t *testing.T, state CeremonyState, flags byte, signCount uint32, userHandle []byte) AuthenticationCredential {
	t.Helper()
	clientData := fixture.clientData(t, "webauthn.get", state.challenge, "https://example.com")
	authData := assertionAuthData("example.com", flags, signCount)
	clientHash := sha256.Sum256(clientData)
	signed := append(append([]byte(nil), authData...), clientHash[:]...)
	digest := sha256.Sum256(signed)
	signature, err := ecdsa.SignASN1(rand.Reader, fixture.privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	encodedID := base64.RawURLEncoding.EncodeToString(fixture.credentialID)
	response := AuthenticationCredential{
		ID: encodedID, RawID: encodedID, Type: "public-key",
		Response: AuthenticationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
			Signature:         base64.RawURLEncoding.EncodeToString(signature),
		},
	}
	if userHandle != nil {
		response.Response.UserHandle = base64.RawURLEncoding.EncodeToString(userHandle)
	}
	return response
}

func (fixture *passkeyFixture) clientData(t *testing.T, ceremonyType, challenge, origin string) []byte {
	t.Helper()
	data, err := json.Marshal(clientData{Type: ceremonyType, Challenge: challenge, Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRegistrationAndAuthenticationES256(t *testing.T) {
	fixture := newFixture(t)
	registration := fixture.register(t, flagUP|flagUV|flagAT|flagBE, ES256)
	if registration.Attestation != "none" || registration.Credential.Algorithm != ES256 || !registration.Credential.BackupEligible {
		t.Fatalf("registration = %#v", registration)
	}
	descriptor := CredentialDescriptor{
		Type: "public-key", ID: base64.RawURLEncoding.EncodeToString(registration.Credential.ID),
	}
	_, state, err := fixture.rp.BeginAuthentication(fixture.user.ID, AuthenticationOptions{
		AllowCredentials: []CredentialDescriptor{descriptor}, RequireUserVerification: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := fixture.authenticationResponse(t, state, flagUP|flagUV|flagBE, 1, fixture.user.ID)
	result, err := fixture.rp.FinishAuthentication(state, response, registration.Credential)
	if err != nil {
		t.Fatal(err)
	}
	if result.SignCount != 1 || result.CounterRisk || result.BackupState {
		t.Fatalf("authentication = %#v", result)
	}
}

// TestTwoIndependentAuthenticatorFixtures exercises the same RP with two
// independently generated authenticator key/credential fixtures. Keeping the
// ceremonies separate catches accidental reliance on one credential's state.
func TestTwoIndependentAuthenticatorFixtures(t *testing.T) {
	first := newFixture(t)
	second := newFixture(t)
	second.credentialID = bytes.Repeat([]byte{0x24}, 32)

	for _, fixture := range []*passkeyFixture{first, second} {
		registration := fixture.register(t, flagUP|flagUV|flagAT|flagBE, ES256)
		descriptor := CredentialDescriptor{
			Type: "public-key", ID: base64.RawURLEncoding.EncodeToString(registration.Credential.ID),
		}
		_, state, err := fixture.rp.BeginAuthentication(fixture.user.ID, AuthenticationOptions{
			AllowCredentials: []CredentialDescriptor{descriptor}, RequireUserVerification: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		response := fixture.authenticationResponse(t, state, flagUP|flagUV|flagBE, 1, fixture.user.ID)
		if _, err := fixture.rp.FinishAuthentication(state, response, registration.Credential); err != nil {
			t.Fatal(err)
		}
	}
}

// TestW3CLevel3NoneES256Vector verifies the independent WebAuthn Level 3
// section 16.2 registration and authentication vector.
func TestW3CLevel3NoneES256Vector(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	rp, err := New(Config{
		RPID: "example.org", RPName: "Example", Origins: []string{"https://example.org"},
		Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	userHandle := []byte("w3c-user")
	credentialID := mustHex(t, "f91f391db4c9b2fde0ea70189cba3fb63f579ba6122b33ad94ff3ec330084be4")
	encodedID := base64.RawURLEncoding.EncodeToString(credentialID)
	registrationState := CeremonyState{
		kind:      registrationCeremony,
		challenge: base64.RawURLEncoding.EncodeToString(mustHex(t, "00c30fb78531c464d2b6771dab8d7b603c01162f2fa486bea70f283ae556e130")),
		expiresAt: now.Add(time.Minute), userHandle: userHandle,
	}
	registration, err := rp.FinishRegistration(registrationState, RegistrationCredential{
		ID: encodedID, RawID: encodedID, Type: "public-key",
		Response: RegistrationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(mustHex(t, "7b2274797065223a22776562617574686e2e637265617465222c226368616c6c656e6765223a22414d4d507434557878475453746e63647134313759447742466938767049612d7077386f4f755657345441222c226f726967696e223a2268747470733a2f2f6578616d706c652e6f7267222c2263726f73734f726967696e223a66616c73652c22657874726144617461223a22636c69656e74446174614a534f4e206d617920626520657874656e6465642077697468206164646974696f6e616c206669656c647320696e20746865206675747572652c207375636820617320746869733a20426b5165446a646354427258426941774a544c453551227d")),
			AttestationObject: base64.RawURLEncoding.EncodeToString(mustHex(t, "a363666d74646e6f6e656761747453746d74a068617574684461746158a4bfabc37432958b063360d3ad6461c9c4735ae7f8edd46592a5e0f01452b2e4b559000000008446ccb9ab1db374750b2367ff6f3a1f0020f91f391db4c9b2fde0ea70189cba3fb63f579ba6122b33ad94ff3ec330084be4a5010203262001215820afefa16f97ca9b2d23eb86ccb64098d20db90856062eb249c33a9b672f26df61225820930a56b87a2fca66334b03458abf879717c12cc68ed73290af2e2664796b9220")),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	authenticationState := CeremonyState{
		kind:      authenticationCeremony,
		challenge: base64.RawURLEncoding.EncodeToString(mustHex(t, "39c0e7521417ba54d43e8dc95174f423dee9bf3cd804ff6d65c857c9abf4d408")),
		expiresAt: now.Add(time.Minute), userHandle: userHandle,
	}
	result, err := rp.FinishAuthentication(authenticationState, AuthenticationCredential{
		ID: encodedID, RawID: encodedID, Type: "public-key",
		Response: AuthenticationCredentialResponse{
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(mustHex(t, "bfabc37432958b063360d3ad6461c9c4735ae7f8edd46592a5e0f01452b2e4b51900000000")),
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(mustHex(t, "7b2274797065223a22776562617574686e2e676574222c226368616c6c656e6765223a224f63446e55685158756c5455506f334a5558543049393770767a7a59425039745a63685879617630314167222c226f726967696e223a2268747470733a2f2f6578616d706c652e6f7267222c2263726f73734f726967696e223a66616c73657d")),
			Signature:         base64.RawURLEncoding.EncodeToString(mustHex(t, "3046022100f50a4e2e4409249c4a853ba361282f09841df4dd4547a13a87780218deffcd380221008480ac0f0b93538174f575bf11a1dd5d78c6e486013f937295ea13653e331e87")),
		},
	}, registration.Credential)
	if err != nil {
		t.Fatal(err)
	}
	if result.CounterRisk || !result.BackupState {
		t.Fatalf("authentication = %#v", result)
	}
}

func TestDiscoverableAuthenticationRequiresUserHandle(t *testing.T) {
	fixture := newFixture(t)
	registration := fixture.register(t, flagUP|flagUV|flagAT, ES256)
	_, state, err := fixture.rp.BeginAuthentication(nil, AuthenticationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	response := fixture.authenticationResponse(t, state, flagUP, 1, nil)
	if _, err := fixture.rp.FinishAuthentication(state, response, registration.Credential); !errors.Is(err, ErrUser) {
		t.Fatalf("missing user handle error = %v", err)
	}
	response = fixture.authenticationResponse(t, state, flagUP, 1, fixture.user.ID)
	if _, err := fixture.rp.FinishAuthentication(state, response, registration.Credential); err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticationNegativeVectors(t *testing.T) {
	fixture := newFixture(t)
	registration := fixture.register(t, flagUP|flagUV|flagAT|flagBE, ES256)
	_, state, err := fixture.rp.BeginAuthentication(fixture.user.ID, AuthenticationOptions{RequireUserVerification: true})
	if err != nil {
		t.Fatal(err)
	}
	valid := fixture.authenticationResponse(t, state, flagUP|flagUV|flagBE, 2, fixture.user.ID)
	t.Run("challenge", func(t *testing.T) {
		badState := state
		badState.challenge = "different"
		if _, err := fixture.rp.FinishAuthentication(badState, valid, registration.Credential); !errors.Is(err, ErrChallenge) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("rp ID hash", func(t *testing.T) {
		bad := valid
		raw, _ := base64.RawURLEncoding.DecodeString(bad.Response.AuthenticatorData)
		raw[0] ^= 1
		bad.Response.AuthenticatorData = base64.RawURLEncoding.EncodeToString(raw)
		if _, err := fixture.rp.FinishAuthentication(state, bad, registration.Credential); !errors.Is(err, ErrRPID) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("origin", func(t *testing.T) {
		bad := valid
		clientData := fixture.clientData(t, "webauthn.get", state.challenge, "https://login.example.com")
		bad.Response.ClientDataJSON = base64.RawURLEncoding.EncodeToString(clientData)
		if _, err := fixture.rp.FinishAuthentication(state, bad, registration.Credential); !errors.Is(err, ErrOrigin) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("flags", func(t *testing.T) {
		bad := fixture.authenticationResponse(t, state, flagUV|flagBE, 2, fixture.user.ID)
		if _, err := fixture.rp.FinishAuthentication(state, bad, registration.Credential); !errors.Is(err, ErrFlags) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("user handle", func(t *testing.T) {
		bad := fixture.authenticationResponse(t, state, flagUP|flagUV|flagBE, 2, []byte("other"))
		if _, err := fixture.rp.FinishAuthentication(state, bad, registration.Credential); !errors.Is(err, ErrUser) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("signature", func(t *testing.T) {
		bad := valid
		signature, _ := base64.RawURLEncoding.DecodeString(bad.Response.Signature)
		signature[len(signature)-1] ^= 1
		bad.Response.Signature = base64.RawURLEncoding.EncodeToString(signature)
		if _, err := fixture.rp.FinishAuthentication(state, bad, registration.Credential); !errors.Is(err, ErrSignature) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("backup eligibility", func(t *testing.T) {
		bad := fixture.authenticationResponse(t, state, flagUP|flagUV, 2, fixture.user.ID)
		if _, err := fixture.rp.FinishAuthentication(state, bad, registration.Credential); !errors.Is(err, ErrFlags) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("counter", func(t *testing.T) {
		credential := registration.Credential
		credential.SignCount = 2
		result, err := fixture.rp.FinishAuthentication(state, valid, credential)
		if err != nil {
			t.Fatal(err)
		}
		if !result.CounterRisk {
			t.Fatal("CounterRisk is false")
		}
	})
}

func TestStrictResponseJSONAndOriginScope(t *testing.T) {
	fixture := newFixture(t)
	if _, err := fixture.rp.DecodeRegistrationCredential([]byte(`{"id":"credential","rawId":"credential","type":"public-key","response":{}}`)); err != nil {
		t.Fatalf("valid registration response error = %v", err)
	}
	if _, err := fixture.rp.DecodeAuthenticationCredential([]byte(`{"id":"credential","rawId":"credential","type":"public-key","response":{}}`)); err != nil {
		t.Fatalf("valid authentication response error = %v", err)
	}
	if _, err := fixture.rp.DecodeAuthenticationCredential([]byte(strings.Repeat("x", fixture.rp.maxJSONBytes+1))); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("oversized response error = %v", err)
	}
	if _, err := fixture.rp.DecodeAuthenticationCredential([]byte(
		`{"id":"one","id":"two","rawId":"one","type":"public-key","response":{}}`,
	)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := New(Config{
		RPID: "other.example", RPName: "Example", Origins: []string{"https://example.com"},
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("RP scope error = %v", err)
	}
	if _, err := New(Config{
		RPID: "example.com", RPName: "Example", Origins: []string{"https://:443"},
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("empty-host origin error = %v", err)
	}
}

func TestCeremonyExpiryAndBackupFlags(t *testing.T) {
	fixture := newFixture(t)
	_, state, err := fixture.rp.BeginAuthentication(fixture.user.ID, AuthenticationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	state.expiresAt = fixture.now
	if _, err := fixture.rp.FinishAuthentication(state, AuthenticationCredential{}, CredentialRecord{}); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry error = %v", err)
	}

	registration := fixture.register(t, flagUP|flagUV|flagAT, ES256)
	_, state, err = fixture.rp.BeginAuthentication(fixture.user.ID, AuthenticationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	response := fixture.authenticationResponse(t, state, flagUP|flagBS, 1, fixture.user.ID)
	if _, err := fixture.rp.FinishAuthentication(state, response, registration.Credential); !errors.Is(err, ErrFlags) {
		t.Fatalf("backup flag error = %v", err)
	}
}

func TestRegistrationRejectsUnsupportedAlgorithm(t *testing.T) {
	fixture := newFixture(t)
	creation, state, err := fixture.rp.BeginRegistration(fixture.user, RegistrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	clientData := fixture.clientData(t, "webauthn.create", creation.Challenge, "https://example.com")
	cose := encodeCOSEKey(t, &fixture.privateKey.PublicKey, -257)
	authData := registrationAuthData("example.com", flagUP|flagAT, 0, fixture.credentialID, cose)
	id := base64.RawURLEncoding.EncodeToString(fixture.credentialID)
	_, err = fixture.rp.FinishRegistration(state, RegistrationCredential{
		ID: id, RawID: id, Type: "public-key", Response: RegistrationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AttestationObject: base64.RawURLEncoding.EncodeToString(encodeAttestationObject(t, authData)),
		},
	})
	if !errors.Is(err, ErrAlgorithm) {
		t.Fatalf("error = %v", err)
	}
}

func TestSessionFlowConsumesStateOnce(t *testing.T) {
	fixture := newFixture(t)
	store, err := memory.NewStore[CeremonyState](memory.Options{Now: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	flow, err := NewSessionFlow(fixture.rp, store)
	if err != nil {
		t.Fatal(err)
	}
	creation, key, err := flow.BeginRegistration(context.Background(), fixture.user, RegistrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	clientData := fixture.clientData(t, "webauthn.create", creation.Challenge, "https://example.com")
	cose := encodeCOSEKey(t, &fixture.privateKey.PublicKey, ES256)
	authData := registrationAuthData("example.com", flagUP|flagAT, 0, fixture.credentialID, cose)
	id := base64.RawURLEncoding.EncodeToString(fixture.credentialID)
	response := RegistrationCredential{
		ID: id, RawID: id, Type: "public-key", Response: RegistrationCredentialResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString(clientData),
			AttestationObject: base64.RawURLEncoding.EncodeToString(encodeAttestationObject(t, authData)),
		},
	}
	registration, err := flow.FinishRegistration(context.Background(), key, response)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := flow.FinishRegistration(context.Background(), key, response); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("replay error = %v", err)
	}
	authenticationRequest, authenticationKey, err := flow.BeginAuthentication(context.Background(), fixture.user.ID, AuthenticationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	authenticationState := CeremonyState{challenge: authenticationRequest.Challenge}
	authenticationResponse := fixture.authenticationResponse(t, authenticationState, flagUP, 1, fixture.user.ID)
	if _, err := flow.FinishAuthentication(context.Background(), authenticationKey, authenticationResponse, registration.Credential); err != nil {
		t.Fatal(err)
	}
	if _, err := flow.FinishAuthentication(context.Background(), authenticationKey, authenticationResponse, registration.Credential); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("authentication replay error = %v", err)
	}

	creation, concurrentKey, err := flow.BeginRegistration(context.Background(), fixture.user, RegistrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	clientData = fixture.clientData(t, "webauthn.create", creation.Challenge, "https://example.com")
	concurrentResponse := response
	concurrentResponse.Response.ClientDataJSON = base64.RawURLEncoding.EncodeToString(clientData)
	var successes atomic.Int32
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := flow.FinishRegistration(context.Background(), concurrentKey, concurrentResponse); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrInvalidState) {
				t.Errorf("concurrent finish error = %v", err)
			}
		}()
	}
	wait.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("concurrent finish successes = %d, want 1", got)
	}
}

func TestSessionFlowRejectsNilInputs(t *testing.T) {
	fixture := newFixture(t)
	store, err := memory.NewStore[CeremonyState](memory.Options{})
	if err != nil {
		t.Fatal(err)
	}
	flow, err := NewSessionFlow(fixture.rp, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := flow.BeginRegistration(nil, fixture.user, RegistrationOptions{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("nil BeginRegistration error = %v", err)
	}
	if _, err := flow.FinishRegistration(nil, "key", RegistrationCredential{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("nil FinishRegistration error = %v", err)
	}
	if _, _, err := flow.BeginAuthentication(nil, nil, AuthenticationOptions{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("nil BeginAuthentication error = %v", err)
	}
	if _, err := flow.FinishAuthentication(nil, "key", AuthenticationCredential{}, CredentialRecord{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("nil FinishAuthentication error = %v", err)
	}
	var nilFlow *SessionFlow
	if _, _, err := nilFlow.BeginRegistration(context.Background(), fixture.user, RegistrationOptions{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("nil flow BeginRegistration error = %v", err)
	}
}

func encodeCOSEKey(t *testing.T, key *ecdsa.PublicKey, algorithm int) []byte {
	t.Helper()
	x := key.X.FillBytes(make([]byte, 32))
	y := key.Y.FillBytes(make([]byte, 32))
	return encodeMap(t, []cbor.MapEntry{
		{Key: encodeInt(t, 1), Value: encodeInt(t, 2)},
		{Key: encodeInt(t, 3), Value: encodeInt(t, int64(algorithm))},
		{Key: encodeInt(t, -1), Value: encodeInt(t, 1)},
		{Key: encodeInt(t, -2), Value: encodeBytes(t, x)},
		{Key: encodeInt(t, -3), Value: encodeBytes(t, y)},
	})
}

func encodeAttestationObject(t *testing.T, authData []byte) []byte {
	t.Helper()
	emptyMap := encodeMap(t, nil)
	return encodeMap(t, []cbor.MapEntry{
		{Key: encodeText(t, "fmt"), Value: encodeText(t, "none")},
		{Key: encodeText(t, "authData"), Value: encodeBytes(t, authData)},
		{Key: encodeText(t, "attStmt"), Value: emptyMap},
	})
}

func registrationAuthData(rpID string, flags byte, signCount uint32, credentialID, cose []byte) []byte {
	hash := sha256.Sum256([]byte(rpID))
	result := append([]byte(nil), hash[:]...)
	result = append(result, flags)
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], signCount)
	result = append(result, count[:]...)
	result = append(result, make([]byte, 16)...)
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(credentialID)))
	result = append(result, length[:]...)
	result = append(result, credentialID...)
	return append(result, cose...)
}

func assertionAuthData(rpID string, flags byte, signCount uint32) []byte {
	hash := sha256.Sum256([]byte(rpID))
	result := append([]byte(nil), hash[:]...)
	result = append(result, flags)
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], signCount)
	return append(result, count[:]...)
}

func encodeInt(t *testing.T, value int64) cbor.RawMessage {
	t.Helper()
	return encodeItem(t, func(encoder *cbor.Encoder) error { return encoder.WriteInt(value) })
}

func encodeBytes(t *testing.T, value []byte) cbor.RawMessage {
	t.Helper()
	return encodeItem(t, func(encoder *cbor.Encoder) error { return encoder.WriteBytes(value) })
}

func encodeText(t *testing.T, value string) cbor.RawMessage {
	t.Helper()
	return encodeItem(t, func(encoder *cbor.Encoder) error { return encoder.WriteText(value) })
}

func encodeMap(t *testing.T, entries []cbor.MapEntry) cbor.RawMessage {
	t.Helper()
	return encodeItem(t, func(encoder *cbor.Encoder) error { return encoder.WriteMap(entries) })
}

func encodeItem(t *testing.T, write func(*cbor.Encoder) error) cbor.RawMessage {
	t.Helper()
	var buffer bytes.Buffer
	encoder, err := cbor.NewEncoder(&buffer, cbor.EncoderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := write(encoder); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func mustHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
