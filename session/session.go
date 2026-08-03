// Package session provides opaque login sessions and the typed browser cookies
// an application writes on its own. The browser receives only a random token;
// every authoritative lifetime and payload stays in a Store implementation
// owned by the application deployment.
//
// Handlers normally read sessions through Read and never observe the token, the
// store key hash, or the backend client.
//
// Jar reads and writes one typed application cookie, plain, signed, or sealed,
// under one API that does not change with the protection. CookieStore is the
// Store implementation for a deployment that wants no session storage at all;
// every other store keeps its records on the server.
package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

var (
	ErrNotFound       = errors.New("session: record not found")
	ErrExpired        = errors.New("session: record expired")
	ErrCodec          = errors.New("session: codec failure")
	ErrUnavailable    = errors.New("session: backend unavailable")
	ErrInvalidOptions = errors.New("session: invalid options")
	ErrInvalidKey     = errors.New("session: invalid key")
	ErrNoSession      = errors.New("session: no session on request")
)

// Record is the stored session state. Stores persist it under a key hash and
// must treat every field as immutable once written.
type Record[T any] struct {
	// Data is the typed application payload.
	Data T
	// CreatedAt is when this record was written.
	CreatedAt time.Time
	// AuthenticatedAt is when the current authentication strength was
	// established. Rotation after login refreshes it.
	AuthenticatedAt time.Time
	// LastSeenAt is the last renewal timestamp.
	LastSeenAt time.Time
	// ExpiresAt is the authoritative absolute expiry.
	ExpiresAt time.Time
	// IdleExpiresAt is the optional inactivity expiry. It is never later than
	// ExpiresAt.
	IdleExpiresAt time.Time
	// Method records how the session was authenticated, such as oidc.
	Method string
	// Version invalidates records after an incompatible schema or policy
	// change.
	Version int
	// CSRFSecret is the per-session secret policy:csrf-protection validates
	// against. It is written at creation and replaced by rotation, so a token
	// minted for an earlier authentication strength cannot be presented after
	// one.
	//
	// It is deliberately absent from the request view: a handler has no reason
	// to read it, and every path that legitimately needs it reaches it through
	// pwruntime rather than through the session payload.
	CSRFSecret string
}

// Store persists session records behind one context-aware contract. Stores
// never accept or return the raw cookie token.
type Store[T any] interface {
	// Put replaces one key atomically.
	Put(ctx context.Context, keyHash string, record Record[T]) error
	// Get returns ErrNotFound for a missing key and ErrExpired for a record
	// past its absolute or idle expiry.
	Get(ctx context.Context, keyHash string) (Record[T], error)
	// Touch renews an existing record. It never revives a missing or expired
	// one.
	Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error
	// Delete is idempotent.
	Delete(ctx context.Context, keyHash string) error
}

// RequestBinder is the optional contract of a Store whose records live in the
// browser instead of a backend, such as CookieStore. The Manager calls
// BindRequest before every store call it makes on behalf of a request, so the
// store can reach the cookie it reads and the response it writes.
//
// A backend store implements nothing here and is unaffected, which is what
// keeps one Manager, one Options, and one handler working over a cookie, an
// RDB, or a Redis store.
type RequestBinder interface {
	BindRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context
}

// Codec serializes the typed payload of a record for durable stores. Record
// timestamps stay in backend fields so renewal never rewrites the payload.
// Implementations must use a bounded format and must not include payload
// contents in returned errors.
type Codec[T any] interface {
	Encode(value T) ([]byte, error)
	Decode(encoded []byte) (T, error)
}

// View is the safe request view of the current session. It excludes the cookie
// token, the store key hash, and backend serialization.
type View[T any] struct {
	Data            T
	CreatedAt       time.Time
	AuthenticatedAt time.Time
	LastSeenAt      time.Time
	ExpiresAt       time.Time
	IdleExpiresAt   time.Time
	Method          string
	Version         int
}

// Read returns the validated session installed by session middleware. It
// reports false for an unauthenticated request and for a payload of another
// type.
func Read[T any](ctx context.Context) (View[T], bool) {
	stored, ok := pwruntime.Session(ctx)
	if !ok {
		return View[T]{}, false
	}
	data, ok := stored.Data.(T)
	if !ok {
		return View[T]{}, false
	}
	return View[T]{
		Data:            data,
		CreatedAt:       stored.CreatedAt,
		AuthenticatedAt: stored.AuthenticatedAt,
		LastSeenAt:      stored.LastSeenAt,
		ExpiresAt:       stored.ExpiresAt,
		IdleExpiresAt:   stored.IdleExpiresAt,
		Method:          stored.Method,
		Version:         stored.Version,
	}, true
}

func (r Record[T]) view() pwruntime.SessionView {
	return pwruntime.SessionView{
		Data:            r.Data,
		CreatedAt:       r.CreatedAt,
		AuthenticatedAt: r.AuthenticatedAt,
		LastSeenAt:      r.LastSeenAt,
		ExpiresAt:       r.ExpiresAt,
		IdleExpiresAt:   r.IdleExpiresAt,
		Method:          r.Method,
		Version:         r.Version,
	}
}

// deadline is the earliest authoritative expiry of the record.
func (r Record[T]) deadline() time.Time {
	if !r.IdleExpiresAt.IsZero() && r.IdleExpiresAt.Before(r.ExpiresAt) {
		return r.IdleExpiresAt
	}
	return r.ExpiresAt
}
