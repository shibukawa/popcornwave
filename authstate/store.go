// Package authstate provides expiring, single-use state storage for browser
// authentication flows.
package authstate

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("authstate: state not found")
	ErrExpired        = errors.New("authstate: state expired")
	ErrAlreadyExists  = errors.New("authstate: state already exists")
	ErrLimitExceeded  = errors.New("authstate: store limit exceeded")
	ErrInvalidKey     = errors.New("authstate: invalid key")
	ErrInvalidExpiry  = errors.New("authstate: invalid expiry")
	ErrInvalidOptions = errors.New("authstate: invalid options")
	ErrCodec          = errors.New("authstate: codec failure")
	ErrUnavailable    = errors.New("authstate: backend unavailable")
)

// Store atomically persists and consumes expiring authentication state.
// Implementations must remove a value before a successful Take returns.
type Store[T any] interface {
	Put(ctx context.Context, key string, value T, expiresAt time.Time) error
	Take(ctx context.Context, key string) (T, error)
}

// Codec explicitly serializes values stored by durable Store implementations.
// Implementations must use a versioned, bounded format and must not include
// value contents in returned errors.
type Codec[T any] interface {
	Encode(value T) ([]byte, error)
	Decode(encoded []byte) (T, error)
}
