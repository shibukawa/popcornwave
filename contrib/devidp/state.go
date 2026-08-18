package devidp

import (
	"errors"
	"fmt"
	"time"

	"github.com/shibukawa/popcornweb/contrib/internal/authn"
)

var errNotRegistered = errors.New("devidp: redirect_uri is not registered")

// storePending records a validated authorization request and returns its key.
func (p *Provider) storePending(pending *pendingAuthorization) (string, error) {
	key, err := authn.GenerateSecret(p.random, secretBytes)
	if err != nil {
		return "", err
	}
	csrf, err := authn.GenerateSecret(p.random, secretBytes)
	if err != nil {
		return "", err
	}
	pending.csrf = csrf
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", ErrClosed
	}
	p.sweepLocked()
	if len(p.pending) >= maxPendingAuthorizations {
		return "", fmt.Errorf("devidp: too many pending authorizations")
	}
	p.pending[key] = pending
	return key, nil
}

// peekPending returns an unexpired pending authorization without consuming it.
func (p *Provider) peekPending(key string) *pendingAuthorization {
	if key == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	pending := p.pending[key]
	if pending == nil {
		return nil
	}
	if p.now().After(pending.expiresAt) {
		delete(p.pending, key)
		return nil
	}
	return pending
}

// takePending consumes a pending authorization exactly once.
func (p *Provider) takePending(key string) *pendingAuthorization {
	p.mu.Lock()
	defer p.mu.Unlock()
	pending := p.pending[key]
	delete(p.pending, key)
	if pending == nil || p.now().After(pending.expiresAt) {
		return nil
	}
	return pending
}

// issueCode consumes the pending authorization and stores the code atomically,
// so a resubmitted selection cannot produce a second code.
func (p *Provider) issueCode(pendingKey string, code *issuedCode) (string, error) {
	value, err := authn.GenerateSecret(p.random, secretBytes)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", ErrClosed
	}
	pending := p.pending[pendingKey]
	if pending == nil || p.now().After(pending.expiresAt) {
		delete(p.pending, pendingKey)
		return "", fmt.Errorf("devidp: the authorization expired")
	}
	delete(p.pending, pendingKey)
	p.sweepLocked()
	if len(p.codes) >= maxIssuedCodes {
		return "", fmt.Errorf("devidp: too many outstanding codes")
	}
	p.codes[value] = code
	return value, nil
}

// takeCode consumes an authorization code exactly once.
func (p *Provider) takeCode(value string) *issuedCode {
	if value == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	code := p.codes[value]
	delete(p.codes, value)
	return code
}

func (p *Provider) issueAccessToken(code *issuedCode, now time.Time) (string, error) {
	value, err := authn.GenerateSecret(p.random, secretBytes)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", ErrClosed
	}
	p.sweepLocked()
	if len(p.tokens) >= maxAccessTokens {
		return "", fmt.Errorf("devidp: too many outstanding access tokens")
	}
	p.tokens[value] = &accessToken{
		clientID:  code.clientID,
		subject:   code.subject,
		scopes:    code.scopes,
		expiresAt: now.Add(p.tokenTTL),
	}
	return value, nil
}

func (p *Provider) lookupAccessToken(value string) *accessToken {
	p.mu.Lock()
	defer p.mu.Unlock()
	token := p.tokens[value]
	if token == nil {
		return nil
	}
	if p.now().After(token.expiresAt) {
		delete(p.tokens, value)
		return nil
	}
	return token
}

// revokeTokens drops every access token of one client, restricted to one
// subject when the caller proved which user is signing out.
func (p *Provider) revokeTokens(clientID, subject string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, token := range p.tokens {
		if token.clientID != clientID {
			continue
		}
		if subject != "" && token.subject != subject {
			continue
		}
		delete(p.tokens, key)
	}
}

func (p *Provider) client(id string) *Client {
	if id == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.clients[id]
}

func (p *Provider) user(subject string) *User {
	if subject == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bySubject[subject]
}

// sweepLocked drops expired state so a long developer session stays bounded.
func (p *Provider) sweepLocked() {
	now := p.now()
	for key, pending := range p.pending {
		if now.After(pending.expiresAt) {
			delete(p.pending, key)
		}
	}
	for key, code := range p.codes {
		if now.After(code.expiresAt) {
			delete(p.codes, key)
		}
	}
	for key, token := range p.tokens {
		if now.After(token.expiresAt) {
			delete(p.tokens, key)
		}
	}
	for key, device := range p.devicesByCode {
		if !now.Before(device.expiresAt) {
			delete(p.devicesByCode, key)
			delete(p.devicesByUserCode, device.userCode)
		}
	}
	for source, attempts := range p.verificationAttempts {
		if now.Sub(attempts.startedAt) >= time.Minute {
			delete(p.verificationAttempts, source)
		}
	}
}
