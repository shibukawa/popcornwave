package authstate

import (
	"context"
	"fmt"
	"time"
)

// RawStore is Store over already encoded payloads.
//
// It exists so a storage backend can open a ceremony store for a value type it
// cannot name. plugin/auth keeps three kinds of ceremony record, two of whose
// types are unexported, so a backend package could never construct a
// Store[T] for them; it supplies this instead and the host adds the codec back
// with Typed.
//
// The guarantees are the ones Store states: Take removes a value before
// returning it, an expired value is removed and never returned, and a stable
// error reveals nothing about what was stored.
type RawStore interface {
	Put(ctx context.Context, key string, payload []byte, expiresAt time.Time) error
	Take(ctx context.Context, key string) ([]byte, error)
}

// Typed puts the codec back on a RawStore, producing the contract callers use.
func Typed[T any](raw RawStore, codec Codec[T]) Store[T] {
	return typedStore[T]{raw: raw, codec: codec}
}

type typedStore[T any] struct {
	raw   RawStore
	codec Codec[T]
}

func (s typedStore[T]) Put(ctx context.Context, key string, value T, expiresAt time.Time) error {
	if s.raw == nil || s.codec == nil {
		return ErrInvalidOptions
	}
	payload, err := s.codec.Encode(value)
	if err != nil || len(payload) == 0 {
		return fmt.Errorf("%w: encode", ErrCodec)
	}
	return s.raw.Put(ctx, key, payload, expiresAt)
}

func (s typedStore[T]) Take(ctx context.Context, key string) (T, error) {
	var zero T
	if s.raw == nil || s.codec == nil {
		return zero, ErrInvalidOptions
	}
	payload, err := s.raw.Take(ctx, key)
	if err != nil {
		return zero, err
	}
	value, err := s.codec.Decode(payload)
	if err != nil {
		return zero, fmt.Errorf("%w: decode", ErrCodec)
	}
	return value, nil
}

// RawCodec passes payloads through unchanged, so a typed store can serve as a
// RawStore for a caller that owns the encoding itself.
type RawCodec struct{}

func (RawCodec) Encode(value []byte) ([]byte, error)   { return value, nil }
func (RawCodec) Decode(encoded []byte) ([]byte, error) { return encoded, nil }
