package passkey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
)

const (
	ceremonyStateCodecVersion  = 1
	maxCeremonyStateCodecBytes = 128 << 10
	// maxCeremonyBindingBytes bounds RegistrationOptions.Binding. An account
	// identifier and a label for what kind of principal it is fit easily; the
	// bound is here because the value reaches durable storage.
	maxCeremonyBindingBytes = 256
)

// CeremonyStateCodec serializes private passkey ceremony state for durable
// authstate stores without exposing its fields as public API.
type CeremonyStateCodec struct{}

type ceremonyStateRecord struct {
	Version                 int      `json:"v"`
	Kind                    uint8    `json:"kind"`
	Challenge               string   `json:"challenge"`
	ExpiresAtMS             int64    `json:"expires_at_ms"`
	UserHandle              []byte   `json:"user_handle,omitempty"`
	AllowedCredentialIDs    [][]byte `json:"allowed_credential_ids,omitempty"`
	RequireUserVerification bool     `json:"require_user_verification,omitempty"`
	Binding                 []byte   `json:"binding,omitempty"`
}

func (CeremonyStateCodec) Encode(value CeremonyState) ([]byte, error) {
	if !validCeremonyStateRecord(value) {
		return nil, fmt.Errorf("%w: passkey ceremony encode", authstate.ErrCodec)
	}
	record := ceremonyStateRecord{
		Version: ceremonyStateCodecVersion, Kind: uint8(value.kind), Challenge: value.challenge,
		ExpiresAtMS: value.expiresAt.UnixMilli(), UserHandle: value.userHandle,
		AllowedCredentialIDs:    value.allowedCredentialIDs,
		RequireUserVerification: value.requireUserVerification,
		Binding:                 value.binding,
	}
	encoded, err := json.Marshal(record)
	if err != nil || len(encoded) > maxCeremonyStateCodecBytes {
		return nil, fmt.Errorf("%w: passkey ceremony encode", authstate.ErrCodec)
	}
	return encoded, nil
}

func (CeremonyStateCodec) Decode(encoded []byte) (CeremonyState, error) {
	var zero CeremonyState
	if len(encoded) == 0 || len(encoded) > maxCeremonyStateCodecBytes {
		return zero, fmt.Errorf("%w: passkey ceremony decode", authstate.ErrCodec)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var record ceremonyStateRecord
	if err := decoder.Decode(&record); err != nil {
		return zero, fmt.Errorf("%w: passkey ceremony decode", authstate.ErrCodec)
	}
	if err := requireCeremonyJSONEOF(decoder); err != nil || record.Version != ceremonyStateCodecVersion {
		return zero, fmt.Errorf("%w: passkey ceremony decode", authstate.ErrCodec)
	}
	state := CeremonyState{
		kind: ceremonyKind(record.Kind), challenge: record.Challenge,
		expiresAt:               time.UnixMilli(record.ExpiresAtMS),
		userHandle:              append([]byte(nil), record.UserHandle...),
		allowedCredentialIDs:    cloneCredentialIDs(record.AllowedCredentialIDs),
		requireUserVerification: record.RequireUserVerification,
		binding:                 append([]byte(nil), record.Binding...),
	}
	if !validCeremonyStateRecord(state) {
		return zero, fmt.Errorf("%w: passkey ceremony decode", authstate.ErrCodec)
	}
	return state, nil
}

func validCeremonyStateRecord(state CeremonyState) bool {
	if (state.kind != registrationCeremony && state.kind != authenticationCeremony) ||
		state.challenge == "" || len(state.challenge) > 256 || state.expiresAt.IsZero() ||
		state.expiresAt.UnixMilli() <= 0 || len(state.userHandle) > maxUserHandleBytes ||
		len(state.allowedCredentialIDs) > maxCredentialDescriptors ||
		len(state.binding) > maxCeremonyBindingBytes {
		return false
	}
	for _, id := range state.allowedCredentialIDs {
		if len(id) == 0 || len(id) > defaultMaxCredentialIDBytes {
			return false
		}
	}
	return true
}

func cloneCredentialIDs(values [][]byte) [][]byte {
	if values == nil {
		return nil
	}
	result := make([][]byte, len(values))
	for i := range values {
		result[i] = append([]byte(nil), values[i]...)
	}
	return result
}

func requireCeremonyJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("extra JSON value")
		}
		return err
	}
	return nil
}

var _ authstate.Codec[CeremonyState] = CeremonyStateCodec{}
