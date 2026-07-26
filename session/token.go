package session

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
)

// tokenBytes is the raw entropy of a browser token. policy:session-security
// requires at least 256 bits.
const tokenBytes = 32

// tokenLength is the encoded length of tokenBytes under base64 raw URL
// encoding.
const tokenLength = 43

// newToken returns a canonical opaque browser token.
func newToken(random io.Reader) (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// validToken reports whether value has the exact syntax produced by newToken.
// Rejecting foreign syntax keeps malformed cookies away from the store.
func validToken(value string) bool {
	if len(value) != tokenLength {
		return false
	}
	for index := range len(value) {
		c := value[index]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// keyHash derives the store key from a browser token. The raw token is never
// written to a backend.
func keyHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
