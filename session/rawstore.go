package session

import (
	"context"
	"time"
)

// RawRecord is a Record whose payload is already encoded. It is what a storage
// backend writes and reads, so a backend never sees the application payload
// type and never needs a type parameter.
type RawRecord struct {
	// Payload is the encoded application payload.
	Payload []byte
	// The remaining fields carry the same meaning as in Record.
	CreatedAt       time.Time
	AuthenticatedAt time.Time
	LastSeenAt      time.Time
	ExpiresAt       time.Time
	IdleExpiresAt   time.Time
	Method          string
	Version         int
}

// Deadline is the earliest authoritative expiry of the record. A backend
// enforces it on read, whatever its own expiry mechanism does.
//
// A zero field means "no such bound" rather than "the epoch", so this is the
// earliest of those that are set. See Record.deadline for what reading the
// absolute bound alone used to cost.
func (r RawRecord) Deadline() time.Time {
	if r.ExpiresAt.IsZero() {
		return r.IdleExpiresAt
	}
	if !r.IdleExpiresAt.IsZero() && r.IdleExpiresAt.Before(r.ExpiresAt) {
		return r.IdleExpiresAt
	}
	return r.ExpiresAt
}

// RawStore is the contract a storage backend implements. It is Store without
// the type parameter, which is what lets one registry hold every backend:
// api:session-backend-plugin resolves a backend by name, and the host adds the
// payload type back with Typed.
//
// Every rule of Store applies here unchanged: honor cancellation, never accept
// the raw cookie token, replace one key atomically, never revive an expired
// record, and treat stored expiry as authoritative.
type RawStore interface {
	Put(ctx context.Context, keyHash string, record RawRecord) error
	Get(ctx context.Context, keyHash string) (RawRecord, error)
	Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error
	Delete(ctx context.Context, keyHash string) error
}

// Backend is one opened storage backend together with the responsibilities it
// brings. A host reads capabilities from this value instead of type-asserting
// a plugin type, so adding a backend changes no host.
type Backend struct {
	// Store is required.
	Store RawStore
	// Close releases a client the backend opened. A backend that borrowed a
	// resource from its host leaves it nil: what it did not open, it does not
	// close.
	Close func(context.Context) error
	// Prune is the expiry sweep of a backend that accumulates records. A
	// backend whose server or browser forgets records on its own leaves it
	// nil, and its host schedules nothing.
	Prune func(ctx context.Context, now time.Time, limit int) (int64, error)
}

// Typed adds the payload type back to a RawStore, producing the Store a
// Manager takes. The codec belongs to the host, which is why a backend can be
// selected by configuration without knowing what an application stores.
func Typed[T any](raw RawStore, codec Codec[T]) Store[T] {
	if codec == nil {
		codec = JSONCodec[T]{}
	}
	return typedStore[T]{raw: raw, codec: codec}
}

type typedStore[T any] struct {
	raw   RawStore
	codec Codec[T]
}

func (s typedStore[T]) Put(ctx context.Context, keyHash string, record Record[T]) error {
	payload, err := s.codec.Encode(record.Data)
	if err != nil {
		return err
	}
	return s.raw.Put(ctx, keyHash, RawRecord{
		Payload:         payload,
		CreatedAt:       record.CreatedAt,
		AuthenticatedAt: record.AuthenticatedAt,
		LastSeenAt:      record.LastSeenAt,
		ExpiresAt:       record.ExpiresAt,
		IdleExpiresAt:   record.IdleExpiresAt,
		Method:          record.Method,
		Version:         record.Version,
	})
}

func (s typedStore[T]) Get(ctx context.Context, keyHash string) (Record[T], error) {
	var zero Record[T]
	raw, err := s.raw.Get(ctx, keyHash)
	if err != nil {
		return zero, err
	}
	data, err := s.codec.Decode(raw.Payload)
	if err != nil {
		return zero, err
	}
	return Record[T]{
		Data:            data,
		CreatedAt:       raw.CreatedAt,
		AuthenticatedAt: raw.AuthenticatedAt,
		LastSeenAt:      raw.LastSeenAt,
		ExpiresAt:       raw.ExpiresAt,
		IdleExpiresAt:   raw.IdleExpiresAt,
		Method:          raw.Method,
		Version:         raw.Version,
	}, nil
}

func (s typedStore[T]) Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error {
	return s.raw.Touch(ctx, keyHash, lastSeenAt, idleExpiresAt)
}

func (s typedStore[T]) Delete(ctx context.Context, keyHash string) error {
	return s.raw.Delete(ctx, keyHash)
}

// BindRequest forwards the binding of a store that keeps its records in the
// browser. Wrapping such a store must not hide it from the Manager.
func (s typedStore[T]) BindRequest(ctx context.Context, carrier Carrier) context.Context {
	binder, ok := s.raw.(RequestBinder)
	if !ok {
		return ctx
	}
	return binder.BindRequest(ctx, carrier)
}

var _ RequestBinder = typedStore[string]{}
