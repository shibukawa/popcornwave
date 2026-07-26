package devidp

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// maxIDTokenBytes bounds an id_token_hint before any parsing work.
const maxIDTokenBytes = 8 << 10

var errInvalidToken = errors.New("devidp: invalid token")

// signJWT produces a compact RS256 JWT.
//
// contrib/jwt signs HS256 only, because RS256 signing waits on the TinyGo
// matrix. This provider is host-only, so it signs here instead of widening the
// TinyGo surface of a package that ships inside applications.
func (p *Provider) signJWT(claims map[string]any) (string, error) {
	p.mu.Lock()
	key := p.key
	p.mu.Unlock()
	if key == nil {
		return "", ErrClosed
	}
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": p.kid}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(p.random, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("devidp: sign id token: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// verifyJWT checks a token this provider issued and returns its claims. It
// exists for id_token_hint, where an unverified hint must not be able to end
// somebody else's session.
func (p *Provider) verifyJWT(compact string) (map[string]any, error) {
	p.mu.Lock()
	key := p.key
	p.mu.Unlock()
	if key == nil {
		return nil, ErrClosed
	}
	parts := strings.Split(compact, ".")
	if len(parts) != 3 || len(compact) > maxIDTokenBytes {
		return nil, errInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errInvalidToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, errInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errInvalidToken
	}
	if issuer, _ := claims["iss"].(string); issuer != p.issuer {
		return nil, errInvalidToken
	}
	return claims, nil
}

// claimString reads a string claim.
func claimString(claims map[string]any, name string) string {
	value, _ := claims[name].(string)
	return value
}

// jwksDocument publishes the current public key.
func (p *Provider) jwksDocument() ([]byte, error) {
	p.mu.Lock()
	key := p.key
	p.mu.Unlock()
	if key == nil {
		return nil, ErrClosed
	}
	exponent := big.NewInt(int64(key.PublicKey.E)).Bytes()
	document := map[string]any{"keys": []any{map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": p.kid,
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(exponent),
	}}}
	return json.Marshal(document)
}

// identityClaims collects the claims a granted scope set exposes for a user.
// Roster claims are always present so a developer can exercise application
// logic without also configuring scopes.
func identityClaims(user *User, scopes []string) map[string]any {
	claims := map[string]any{}
	for name, value := range user.Claims {
		claims[name] = value
	}
	if contains(scopes, "profile") {
		if _, ok := claims["name"]; !ok && user.DisplayName != "" {
			claims["name"] = user.DisplayName
		}
		claims["preferred_username"] = user.Key
	}
	if !contains(scopes, "profile") {
		for _, name := range []string{"name", "family_name", "given_name", "preferred_username", "picture", "profile"} {
			delete(claims, name)
		}
	}
	if !contains(scopes, "email") {
		delete(claims, "email")
		delete(claims, "email_verified")
	} else if _, ok := claims["email"]; ok {
		if _, set := claims["email_verified"]; !set {
			claims["email_verified"] = true
		}
	}
	return claims
}

// idToken builds and signs the ID Token for a consumed authorization code.
func (p *Provider) idToken(code *issuedCode, user *User, now time.Time) (string, error) {
	claims := identityClaims(user, code.scopes)
	claims["iss"] = p.issuer
	claims["sub"] = user.Subject
	claims["aud"] = code.clientID
	claims["iat"] = now.Unix()
	claims["auth_time"] = code.issuedAt.Unix()
	claims["exp"] = now.Add(p.tokenTTL).Unix()
	if code.nonce != "" {
		claims["nonce"] = code.nonce
	}
	return p.signJWT(claims)
}
