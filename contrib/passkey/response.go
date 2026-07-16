package passkey

import (
	"encoding/json"
	"errors"

	"github.com/shibukawa/petitweb-go/contrib/internal/authn"
)

// DecodeRegistrationCredential strictly decodes one bounded browser response.
func (rp *RelyingParty) DecodeRegistrationCredential(data []byte) (RegistrationCredential, error) {
	if !rp.ready() {
		return RegistrationCredential{}, ErrInvalidConfig
	}
	if err := rp.validateResponseJSON(data); err != nil {
		return RegistrationCredential{}, err
	}
	var response RegistrationCredential
	if err := json.Unmarshal(data, &response); err != nil {
		return RegistrationCredential{}, ErrMalformed
	}
	return response, nil
}

// DecodeAuthenticationCredential strictly decodes one bounded browser response.
func (rp *RelyingParty) DecodeAuthenticationCredential(data []byte) (AuthenticationCredential, error) {
	if !rp.ready() {
		return AuthenticationCredential{}, ErrInvalidConfig
	}
	if err := rp.validateResponseJSON(data); err != nil {
		return AuthenticationCredential{}, err
	}
	var response AuthenticationCredential
	if err := json.Unmarshal(data, &response); err != nil {
		return AuthenticationCredential{}, ErrMalformed
	}
	return response, nil
}

func (rp *RelyingParty) validateResponseJSON(data []byte) error {
	err := authn.ValidateJSON(data, authn.JSONOptions{
		MaxBytes: rp.maxJSONBytes, MaxDepth: 12, MaxMembers: 128,
	})
	if errors.Is(err, authn.ErrLimitExceeded) {
		return ErrLimitExceeded
	}
	if err != nil {
		return ErrMalformed
	}
	return nil
}
