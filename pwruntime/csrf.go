package pwruntime

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"
)

// CSRFCookieName is the companion cookie the browser runtime reads a token from.
//
// It is a constant rather than configuration because it is a contract between
// the framework and the shipped script, not a deployment choice. The runtime is
// a module script, so it cannot read a name off its own tag the way a classic
// script can; a renamed cookie would leave it looking for one that is not
// there, and nothing would say so.
const CSRFCookieName = "pw_csrf"

// CSRFHeaderName is where the runtime puts the token. It is outside the
// framework's own header namespace because it is a name middleware already
// looks for.
const CSRFHeaderName = "X-CSRF-Token"

// CSRFSecretBytes is the raw entropy of a session's CSRF secret. It matches the
// session token, because both are the same kind of unguessable value.
const CSRFSecretBytes = 32

// csrfTokenBytes is the decoded length of an emitted token: the pad, then the
// pad combined with the secret.
const csrfTokenBytes = CSRFSecretBytes * 2

// NewCSRFSecret returns a session's CSRF secret, encoded the way every other
// opaque value in this framework is.
func NewCSRFSecret(random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, CSRFSecretBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// CSRFToken derives the value a browser receives from the session's secret.
//
// The pad is fresh for every call, and the emitted bytes therefore differ on
// every response. That is the whole point: a stable token reflected into a
// compressed body beside attacker-influenced input is what a compression oracle
// extracts one byte at a time, which is why Rails and Django both mask.
//
// The pad travels in the clear because it has to: verification recomputes the
// expected value from it. Knowing the pad reveals nothing, since producing the
// rest of the token still requires the secret.
func CSRFToken(secret string, random io.Reader) (string, error) {
	raw, ok := decodeCSRFSecret(secret)
	if !ok {
		return "", nil
	}
	if random == nil {
		random = rand.Reader
	}
	pad := make([]byte, CSRFSecretBytes)
	if _, err := io.ReadFull(random, pad); err != nil {
		return "", err
	}
	return encodeCSRFToken(pad, raw), nil
}

// ExpectedCSRFToken rebuilds the token a correct client would have sent, using
// the pad carried by the one that arrived.
//
// The comparison stays an equality against a value this framework computed,
// which is what lets the masked form work with a verifier that only compares.
// A malformed token yields an empty string, so it cannot match and cannot panic.
func ExpectedCSRFToken(secret, presented string) string {
	raw, ok := decodeCSRFSecret(secret)
	if !ok {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(presented)
	if err != nil || len(decoded) != csrfTokenBytes {
		return ""
	}
	return encodeCSRFToken(decoded[:CSRFSecretBytes], raw)
}

// VerifyCSRFToken reports whether presented unmasks to secret.
//
// It exists so a caller that does not route through the update options still
// compares in constant time rather than with ==.
func VerifyCSRFToken(secret, presented string) bool {
	expected := ExpectedCSRFToken(secret, presented)
	if expected == "" || presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) == 1
}

func encodeCSRFToken(pad, secret []byte) string {
	out := make([]byte, csrfTokenBytes)
	copy(out, pad)
	for index := range secret {
		out[CSRFSecretBytes+index] = pad[index] ^ secret[index]
	}
	return base64.RawURLEncoding.EncodeToString(out)
}

func decodeCSRFSecret(secret string) ([]byte, bool) {
	if secret == "" {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(raw) != CSRFSecretBytes {
		return nil, false
	}
	return raw, true
}

// csrfSecretKey carries the request's CSRF secret.
//
// It is deliberately not a field of SessionView. A handler reads the view, and
// the secret is the one piece of a session record that must never reach one:
// policy:csrf-protection keeps it out of the request view, out of template
// scope, and out of every log.
type csrfSecretKey struct{}

// WithCSRFSecret carries the session's CSRF secret for the framework code that
// emits and verifies tokens. The session middleware sets it; nothing else does.
func WithCSRFSecret(ctx context.Context, secret string) context.Context {
	if secret == "" {
		return ctx
	}
	return context.WithValue(ctx, csrfSecretKey{}, secret)
}

// CSRFSecret returns the request's CSRF secret, if the request carries a
// session that has one.
func CSRFSecret(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	secret, ok := ctx.Value(csrfSecretKey{}).(string)
	return secret, ok && secret != ""
}
