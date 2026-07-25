// Package memory provides process-local authentication state storage.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/authstate"
)

const (
	defaultMaxEntries  = 4096
	defaultMaxKeyBytes = 256
	// hardMaxEntries and hardMaxKeyBytes prevent a malformed deployment
	// configuration from turning the correlation store into an unbounded
	// allocation surface.
	hardMaxEntries  = 1 << 16
	hardMaxKeyBytes = 4096
)

// Options controls Store resource limits and time behavior. Zero values select
// conservative defaults.
type Options struct {
	Now         func() time.Time
	MaxEntries  int
	MaxKeyBytes int
}

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

// Store is a process-local authstate.Store intended for tests, development,
// and single-process deployments. Values are stored by assignment and must be
// treated as immutable by callers.
type Store[T any] struct {
	mu          sync.Mutex
	entries     map[string]entry[T]
	now         func() time.Time
	maxEntries  int
	maxKeyBytes int
}

// NewStore constructs a bounded process-local authentication state store.
func NewStore[T any](options Options) (*Store[T], error) {
	if options.MaxEntries < 0 || options.MaxEntries > hardMaxEntries ||
		options.MaxKeyBytes < 0 || options.MaxKeyBytes > hardMaxKeyBytes {
		return nil, authstate.ErrInvalidOptions
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
	return &Store[T]{
		entries:     make(map[string]entry[T]),
		now:         now,
		maxEntries:  maxEntries,
		maxKeyBytes: maxKeyBytes,
	}, nil
}

func (s *Store[T]) Put(ctx context.Context, key string, value T, expiresAt time.Time) error {
	if s == nil || s.now == nil || s.entries == nil || ctx == nil {
		return authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return authstate.ErrInvalidKey
	}
	now := s.now()
	if expiresAt.IsZero() || !expiresAt.After(now) {
		return authstate.ErrInvalidExpiry
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.removeExpired(now)
	if _, exists := s.entries[key]; exists {
		return authstate.ErrAlreadyExists
	}
	if len(s.entries) >= s.maxEntries {
		return authstate.ErrLimitExceeded
	}
	s.entries[key] = entry[T]{value: value, expiresAt: expiresAt}
	return nil
}

func (s *Store[T]) Take(ctx context.Context, key string) (T, error) {
	var zero T
	if s == nil || s.now == nil || s.entries == nil || ctx == nil {
		return zero, authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return zero, authstate.ErrInvalidKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	item, exists := s.entries[key]
	if !exists {
		return zero, authstate.ErrNotFound
	}
	delete(s.entries, key)
	if !item.expiresAt.After(s.now()) {
		return zero, authstate.ErrExpired
	}
	return item.value, nil
}

func (s *Store[T]) removeExpired(now time.Time) {
	for key, item := range s.entries {
		if !item.expiresAt.After(now) {
			delete(s.entries, key)
		}
	}
}

var _ authstate.Store[int] = (*Store[int])(nil)
