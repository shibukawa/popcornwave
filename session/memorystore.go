package session

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is a process-local RawStore. It is intended for development and
// tests: records are concurrency-safe and expiry-aware, but disappear when the
// process stops and are never shared with another process.
//
// Environment policy does not belong to the storage package. The built-in pw
// backend that exposes this store rejects it outside development.
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]RawRecord
	now     func() time.Time
}

// NewMemoryStore returns an empty process-local store.
func NewMemoryStore(now func() time.Time) *MemoryStore {
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{records: make(map[string]RawRecord), now: now}
}

func (s *MemoryStore) Put(ctx context.Context, keyHash string, record RawRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKeyHash(keyHash) {
		return ErrInvalidKey
	}
	record.Payload = append([]byte(nil), record.Payload...)
	s.mu.Lock()
	s.records[keyHash] = record
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, keyHash string) (RawRecord, error) {
	if err := ctx.Err(); err != nil {
		return RawRecord{}, err
	}
	if !validKeyHash(keyHash) {
		return RawRecord{}, ErrInvalidKey
	}
	// Reads share the lock; only an expired record needs the write lock to
	// delete itself.
	s.mu.RLock()
	record, ok := s.records[keyHash]
	s.mu.RUnlock()
	if !ok {
		return RawRecord{}, ErrNotFound
	}
	if deadline := record.Deadline(); !deadline.IsZero() && !deadline.After(s.now()) {
		s.mu.Lock()
		delete(s.records, keyHash)
		s.mu.Unlock()
		return RawRecord{}, ErrExpired
	}
	record.Payload = append([]byte(nil), record.Payload...)
	return record, nil
}

func (s *MemoryStore) Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKeyHash(keyHash) {
		return ErrInvalidKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[keyHash]
	if !ok {
		return ErrNotFound
	}
	if deadline := record.Deadline(); !deadline.IsZero() && !deadline.After(s.now()) {
		delete(s.records, keyHash)
		return ErrExpired
	}
	if !record.ExpiresAt.IsZero() && idleExpiresAt.After(record.ExpiresAt) {
		return ErrExpired
	}
	record.LastSeenAt = lastSeenAt
	record.IdleExpiresAt = idleExpiresAt
	s.records[keyHash] = record
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, keyHash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKeyHash(keyHash) {
		return ErrInvalidKey
	}
	s.mu.Lock()
	delete(s.records, keyHash)
	s.mu.Unlock()
	return nil
}

var _ RawStore = (*MemoryStore)(nil)
