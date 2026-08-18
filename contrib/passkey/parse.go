package passkey

import (
	"bytes"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"

	"github.com/shibukawa/popcornweb/contrib/cbor"
	"github.com/shibukawa/popcornweb/contrib/internal/authn"
)

const (
	flagUP  = 1 << 0
	flagUV  = 1 << 2
	flagBE  = 1 << 3
	flagBS  = 1 << 4
	flagAT  = 1 << 6
	flagED  = 1 << 7
	flagRFU = 1<<1 | 1<<5
)

type clientData struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"crossOrigin,omitempty"`
	TopOrigin   string `json:"topOrigin,omitempty"`
}

type authenticatorData struct {
	raw            []byte
	rpIDHash       [32]byte
	flags          byte
	signCount      uint32
	aaguid         [16]byte
	credentialID   []byte
	credentialCOSE []byte
	publicKeyX     []byte
	publicKeyY     []byte
	algorithm      int
}

func (rp *RelyingParty) parseClientData(encoded, expectedType string, state CeremonyState) ([]byte, clientData, error) {
	raw, err := authn.DecodeBase64URL(encoded, encodedLimit(rp.maxJSONBytes), rp.maxJSONBytes)
	if err != nil {
		return nil, clientData{}, classifyInputError(err)
	}
	if err := authn.ValidateJSON(raw, authn.JSONOptions{
		MaxBytes: rp.maxJSONBytes, MaxDepth: 8, MaxMembers: 64,
	}); err != nil {
		return nil, clientData{}, classifyInputError(err)
	}
	var data clientData
	if err := json.Unmarshal(raw, &data); err != nil || data.Type != expectedType || data.CrossOrigin || data.TopOrigin != "" {
		return nil, clientData{}, ErrMalformed
	}
	if !authn.EqualSecret(data.Challenge, state.challenge) {
		return nil, clientData{}, ErrChallenge
	}
	if _, allowed := rp.origins[data.Origin]; !allowed {
		return nil, clientData{}, ErrOrigin
	}
	return raw, data, nil
}

func (rp *RelyingParty) decodeCredential(id, rawID, credentialType string) ([]byte, error) {
	if credentialType != "public-key" || id == "" || rawID == "" {
		return nil, ErrCredential
	}
	decodedID, err := authn.DecodeBase64URL(id, encodedLimit(rp.maxCredentialIDBytes), rp.maxCredentialIDBytes)
	if err != nil || len(decodedID) == 0 {
		return nil, ErrCredential
	}
	decodedRawID, err := authn.DecodeBase64URL(rawID, encodedLimit(rp.maxCredentialIDBytes), rp.maxCredentialIDBytes)
	if err != nil || subtle.ConstantTimeCompare(decodedID, decodedRawID) != 1 {
		return nil, ErrCredential
	}
	return decodedID, nil
}

func (rp *RelyingParty) parseAttestationObject(encoded string) ([]byte, error) {
	raw, err := authn.DecodeBase64URL(encoded, encodedLimit(rp.maxAttestationBytes), rp.maxAttestationBytes)
	if err != nil {
		return nil, classifyInputError(err)
	}
	decoder, err := cbor.NewDecoder(bytes.NewReader(raw), cbor.DecoderOptions{
		MaxInputBytes: int64(rp.maxAttestationBytes), MaxNestedLevels: 8, MaxContainerItems: 32,
		MaxStringBytes: rp.maxAuthenticatorBytes, MaxRawMessageBytes: rp.maxAttestationBytes,
		RejectDuplicateMapKeys: true,
	})
	if err != nil {
		return nil, ErrMalformed
	}
	pairs, indefinite, err := decoder.ReadMap()
	if err != nil || indefinite || pairs != 3 {
		return nil, ErrMalformed
	}
	var format string
	var authData []byte
	attestationEmpty := false
	for range pairs {
		key, err := decoder.ReadText()
		if err != nil {
			return nil, ErrMalformed
		}
		switch key {
		case "fmt":
			format, err = decoder.ReadText()
		case "authData":
			authData, err = decoder.ReadBytes()
		case "attStmt":
			var count int
			var mapIndefinite bool
			count, mapIndefinite, err = decoder.ReadMap()
			if err == nil && !mapIndefinite && count == 0 {
				var token cbor.Token
				token, err = decoder.ReadToken()
				attestationEmpty = err == nil && token.Kind == cbor.EndMap
			}
		default:
			return nil, ErrMalformed
		}
		if err != nil {
			return nil, ErrMalformed
		}
	}
	end, err := decoder.ReadToken()
	if err != nil || end.Kind != cbor.EndMap {
		return nil, ErrMalformed
	}
	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return nil, ErrMalformed
	}
	if format != "none" || !attestationEmpty || len(authData) == 0 {
		return nil, ErrAttestation
	}
	return authData, nil
}

func (rp *RelyingParty) parseAuthenticatorData(raw []byte, registration bool) (authenticatorData, error) {
	if len(raw) < 37 {
		return authenticatorData{}, ErrMalformed
	}
	if len(raw) > rp.maxAuthenticatorBytes {
		return authenticatorData{}, ErrLimitExceeded
	}
	result := authenticatorData{raw: append([]byte(nil), raw...), flags: raw[32], signCount: binary.BigEndian.Uint32(raw[33:37])}
	copy(result.rpIDHash[:], raw[:32])
	expectedRPIDHash := sha256.Sum256([]byte(rp.rpID))
	if subtle.ConstantTimeCompare(result.rpIDHash[:], expectedRPIDHash[:]) != 1 {
		return authenticatorData{}, ErrRPID
	}
	if result.flags&flagRFU != 0 || result.flags&flagUP == 0 || result.flags&flagBS != 0 && result.flags&flagBE == 0 {
		return authenticatorData{}, ErrFlags
	}
	if registration && result.flags&flagAT == 0 {
		return authenticatorData{}, ErrFlags
	}
	if !registration && result.flags&flagAT != 0 {
		return authenticatorData{}, ErrFlags
	}
	offset := 37
	if result.flags&flagAT != 0 {
		if len(raw) < offset+18 {
			return authenticatorData{}, ErrMalformed
		}
		copy(result.aaguid[:], raw[offset:offset+16])
		offset += 16
		credentialLength := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
		offset += 2
		if credentialLength == 0 || credentialLength > rp.maxCredentialIDBytes || len(raw) < offset+credentialLength {
			return authenticatorData{}, ErrLimitExceeded
		}
		result.credentialID = append([]byte(nil), raw[offset:offset+credentialLength]...)
		offset += credentialLength
		reader := bytes.NewReader(raw[offset:])
		decoder, err := cbor.NewDecoder(reader, cbor.DecoderOptions{
			MaxInputBytes: int64(len(raw) - offset), MaxNestedLevels: 4, MaxContainerItems: 16,
			MaxStringBytes: 128, MaxRawMessageBytes: 512, RejectDuplicateMapKeys: true, Sequence: true,
		})
		if err != nil {
			return authenticatorData{}, ErrMalformed
		}
		cose, err := decoder.ReadRaw()
		if err != nil {
			return authenticatorData{}, ErrMalformed
		}
		result.credentialCOSE = append([]byte(nil), cose...)
		offset += len(cose)
		result.publicKeyX, result.publicKeyY, result.algorithm, err = parseCOSEKey(cose)
		if err != nil {
			return authenticatorData{}, err
		}
	}
	remaining := raw[offset:]
	if result.flags&flagED != 0 {
		if len(remaining) == 0 || validateExtensionMap(remaining) != nil {
			return authenticatorData{}, ErrMalformed
		}
	} else if len(remaining) != 0 {
		return authenticatorData{}, ErrMalformed
	}
	return result, nil
}

func parseCOSEKey(raw []byte) ([]byte, []byte, int, error) {
	deterministic, err := cbor.NewEncoder(io.Discard, cbor.EncoderOptions{
		MaxNestedLevels: 4, MaxContainerItems: 10, MaxStringBytes: 64,
	})
	if err != nil || deterministic.WriteRaw(raw) != nil {
		return nil, nil, 0, ErrMalformed
	}
	decoder, err := cbor.NewDecoder(bytes.NewReader(raw), cbor.DecoderOptions{
		MaxInputBytes: int64(len(raw)), MaxNestedLevels: 4, MaxContainerItems: 10,
		MaxStringBytes: 64, MaxRawMessageBytes: 512, RejectDuplicateMapKeys: true,
	})
	if err != nil {
		return nil, nil, 0, ErrMalformed
	}
	pairs, indefinite, err := decoder.ReadMap()
	if err != nil || indefinite || pairs != 5 {
		return nil, nil, 0, ErrMalformed
	}
	values := make(map[int64]any, 5)
	for range pairs {
		label, err := decoder.ReadInt()
		if err != nil {
			return nil, nil, 0, ErrMalformed
		}
		switch label {
		case 1, 3, -1:
			value, err := decoder.ReadInt()
			if err != nil {
				return nil, nil, 0, ErrMalformed
			}
			values[label] = value
		case -2, -3:
			value, err := decoder.ReadBytes()
			if err != nil {
				return nil, nil, 0, ErrMalformed
			}
			values[label] = value
		default:
			return nil, nil, 0, ErrAlgorithm
		}
	}
	end, err := decoder.ReadToken()
	if err != nil || end.Kind != cbor.EndMap {
		return nil, nil, 0, ErrMalformed
	}
	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return nil, nil, 0, ErrMalformed
	}
	kty, ktyOK := values[1].(int64)
	algorithm, algorithmOK := values[3].(int64)
	curve, curveOK := values[-1].(int64)
	x, xOK := values[-2].([]byte)
	y, yOK := values[-3].([]byte)
	if !ktyOK || !algorithmOK || !curveOK || !xOK || !yOK || kty != 2 || algorithm != ES256 || curve != 1 || len(x) != 32 || len(y) != 32 {
		return nil, nil, 0, ErrAlgorithm
	}
	if !elliptic.P256().IsOnCurve(new(big.Int).SetBytes(x), new(big.Int).SetBytes(y)) {
		return nil, nil, 0, ErrAlgorithm
	}
	return append([]byte(nil), x...), append([]byte(nil), y...), int(algorithm), nil
}

func validateExtensionMap(raw []byte) error {
	decoder, err := cbor.NewDecoder(bytes.NewReader(raw), cbor.DecoderOptions{
		MaxInputBytes: int64(len(raw)), MaxNestedLevels: 8, MaxContainerItems: 64,
		MaxStringBytes: len(raw), MaxRawMessageBytes: len(raw), RejectDuplicateMapKeys: true,
	})
	if err != nil {
		return err
	}
	first, err := decoder.ReadToken()
	if err != nil || first.Kind != cbor.StartMap {
		return ErrMalformed
	}
	for {
		_, err = decoder.ReadToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func classifyInputError(err error) error {
	if errors.Is(err, authn.ErrLimitExceeded) {
		return ErrLimitExceeded
	}
	return ErrMalformed
}
