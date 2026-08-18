package passkey

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/shibukawa/popcornweb/authstate"
	"github.com/shibukawa/popcornweb/contrib/internal/authn"
)

// SessionFlow adds atomic single-use state consumption to RelyingParty.
type SessionFlow struct {
	rp    *RelyingParty
	store authstate.Store[CeremonyState]
}

func NewSessionFlow(rp *RelyingParty, store authstate.Store[CeremonyState]) (*SessionFlow, error) {
	if rp == nil || store == nil {
		return nil, ErrInvalidConfig
	}
	return &SessionFlow{rp: rp, store: store}, nil
}

func (flow *SessionFlow) BeginRegistration(ctx context.Context, user User, options RegistrationOptions) (CreationOptions, string, error) {
	if flow == nil || ctx == nil {
		return CreationOptions{}, "", ErrInvalidState
	}
	creation, state, err := flow.rp.BeginRegistration(user, options)
	if err != nil {
		return CreationOptions{}, "", err
	}
	key, err := flow.storeState(ctx, state)
	if err != nil {
		return CreationOptions{}, "", err
	}
	return creation, key, nil
}

// FinishRegistration completes the ceremony begun for binding.
//
// binding must equal the RegistrationOptions.Binding the ceremony started with.
// It is a required argument rather than an option so that a caller cannot leave
// the ceremony unbound by forgetting it: the whole point is that the principal
// resolved at this request may not be the one that began, and an application
// that skipped the comparison would enroll a credential onto whoever happens to
// be authenticated now.
//
// A mismatch consumes the state and fails, exactly as a bad challenge does. The
// ceremony is over either way, and reporting which half was wrong would say
// whether an account is mid-enrollment.
func (flow *SessionFlow) FinishRegistration(ctx context.Context, key string, binding []byte, response RegistrationCredential) (RegistrationResult, error) {
	if flow == nil || ctx == nil {
		return RegistrationResult{}, ErrInvalidState
	}
	state, err := flow.store.Take(ctx, key)
	if err != nil {
		return RegistrationResult{}, ErrInvalidState
	}
	if subtle.ConstantTimeCompare(state.binding, binding) != 1 {
		return RegistrationResult{}, ErrInvalidState
	}
	return flow.rp.FinishRegistration(state, response)
}

func (flow *SessionFlow) BeginAuthentication(ctx context.Context, userHandle []byte, options AuthenticationOptions) (RequestOptions, string, error) {
	if flow == nil || ctx == nil {
		return RequestOptions{}, "", ErrInvalidState
	}
	request, state, err := flow.rp.BeginAuthentication(userHandle, options)
	if err != nil {
		return RequestOptions{}, "", err
	}
	key, err := flow.storeState(ctx, state)
	if err != nil {
		return RequestOptions{}, "", err
	}
	return request, key, nil
}

func (flow *SessionFlow) FinishAuthentication(ctx context.Context, key string, response AuthenticationCredential, credential CredentialRecord) (AuthenticationResult, error) {
	if flow == nil || ctx == nil {
		return AuthenticationResult{}, ErrInvalidState
	}
	state, err := flow.store.Take(ctx, key)
	if err != nil {
		return AuthenticationResult{}, ErrInvalidState
	}
	return flow.rp.FinishAuthentication(state, response, credential)
}

func (flow *SessionFlow) storeState(ctx context.Context, state CeremonyState) (string, error) {
	for range 3 {
		key, err := authn.GenerateSecret(flow.rp.random, 32)
		if err != nil {
			return "", err
		}
		if err := flow.store.Put(ctx, key, state, state.expiresAt); err == nil {
			return key, nil
		} else if !errors.Is(err, authstate.ErrAlreadyExists) {
			return "", err
		}
	}
	return "", authstate.ErrAlreadyExists
}
