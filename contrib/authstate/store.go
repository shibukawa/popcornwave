// Package authstate provides expiring, single-use state storage for browser
// authentication flows.
package authstate

import (
	"context"
	"errors"
	"sync"
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
)

const (
	defaultMaxEntries  = 4096
	defaultMaxKeyBytes = 256
	// hardMaxEntries and hardMaxKeyBytes prevent a malformed deployment
	// configuration from turning the in-memory correlation store into an
	// unbounded allocation surface.
	hardMaxEntries  = 1 << 16
	hardMaxKeyBytes = 4096
)

// Store atomically persists and consumes expiring authentication state.
// Implementations must remove a value before a successful Take returns.
type Store[T any] interface {
	Put(ctx context.Context, key string, value T, expiresAt time.Time) error
	Take(ctx context.Context, key string) (T, error)
}

// Options controls MemoryStore resource limits and time behavior. Zero values
// select conservative defaults.
type Options struct {
	Now         func() time.Time
	MaxEntries  int
	MaxKeyBytes int
}

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

// MemoryStore is a process-local Store intended for tests, development, and
// single-process deployments. Values are stored by assignment and must be
// treated as immutable by callers.
type MemoryStore[T any] struct {
	mu          sync.Mutex
	entries     map[string]entry[T]
	now         func() time.Time
	maxEntries  int
	maxKeyBytes int
}

func NewMemoryStore[T any](options Options) (*MemoryStore[T], error) {
	if options.MaxEntries < 0 || options.MaxEntries > hardMaxEntries ||
		options.MaxKeyBytes < 0 || options.MaxKeyBytes > hardMaxKeyBytes {
		return nil, ErrInvalidOptions
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	maxEntries := options.MaxEntries
	if maxEntries == 0 {
		maxEntries = defaultMaxEntries
	}
	maxKeyBytes := options.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMaxKeyBytes
	}
	return &MemoryStore[T]{
		entries:     make(map[string]entry[T]),
		now:         now,
		maxEntries:  maxEntries,
		maxKeyBytes: maxKeyBytes,
	}, nil
}

func (s *MemoryStore[T]) Put(ctx context.Context, key string, value T, expiresAt time.Time) error {
	if s == nil || s.now == nil || s.entries == nil || ctx == nil {
		return ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return ErrInvalidKey
	}
	now := s.now()
	if expiresAt.IsZero() || !expiresAt.After(now) {
		return ErrInvalidExpiry
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.removeExpired(now)
	if _, exists := s.entries[key]; exists {
		return ErrAlreadyExists
	}
	if len(s.entries) >= s.maxEntries {
		return ErrLimitExceeded
	}
	s.entries[key] = entry[T]{value: value, expiresAt: expiresAt}
	return nil
}

func (s *MemoryStore[T]) Take(ctx context.Context, key string) (T, error) {
	var zero T
	if s == nil || s.now == nil || s.entries == nil || ctx == nil {
		return zero, ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return zero, ErrInvalidKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	item, exists := s.entries[key]
	if !exists {
		return zero, ErrNotFound
	}
	delete(s.entries, key)
	if !item.expiresAt.After(s.now()) {
		return zero, ErrExpired
	}
	return item.value, nil
}

func (s *MemoryStore[T]) removeExpired(now time.Time) {
	for key, item := range s.entries {
		if !item.expiresAt.After(now) {
			delete(s.entries, key)
		}
	}
}

var _ Store[int] = (*MemoryStore[int])(nil)
