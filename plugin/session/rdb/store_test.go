package rdb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/session"

	_ "github.com/shibukawa/tinygodriver/database/sqlite"
)

type payload struct {
	AccountID string `json:"account_id"`
}

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func testStore(t *testing.T, now func() time.Time) (*Store[payload], *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewStore[payload](db, session.JSONCodec[payload]{}, Options{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store, db
}

func TestStoreRoundTripsRecord(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store, _ := testStore(t, func() time.Time { return now })
	ctx := context.Background()

	record := session.Record[payload]{
		Data:            payload{AccountID: "account-1"},
		CreatedAt:       now,
		AuthenticatedAt: now,
		LastSeenAt:      now,
		ExpiresAt:       now.Add(time.Hour),
		IdleExpiresAt:   now.Add(30 * time.Minute),
		Method:          "oidc",
		Version:         1,
	}
	if err := store.Put(ctx, testKey, record); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Data.AccountID != "account-1" || loaded.Method != "oidc" || loaded.Version != 1 {
		t.Fatalf("record = %#v", loaded)
	}
	if !loaded.ExpiresAt.Equal(record.ExpiresAt) || !loaded.IdleExpiresAt.Equal(record.IdleExpiresAt) {
		t.Fatalf("timestamps = %s / %s", loaded.ExpiresAt, loaded.IdleExpiresAt)
	}
}

func TestStoreReplacesOneKeyAtomically(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store, db := testStore(t, func() time.Time { return now })
	ctx := context.Background()
	record := session.Record[payload]{
		Data: payload{AccountID: "first"}, CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := store.Put(ctx, testKey, record); err != nil {
		t.Fatal(err)
	}
	record.Data = payload{AccountID: "second"}
	if err := store.Put(ctx, testKey, record); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM popcornwave_session`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rows = %d", count)
	}
	loaded, err := store.Get(ctx, testKey)
	if err != nil || loaded.Data.AccountID != "second" {
		t.Fatalf("record = %#v err = %v", loaded, err)
	}
}

func TestStoreExpiryIsAuthoritative(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	store, _ := testStore(t, func() time.Time { return current })
	ctx := context.Background()
	if err := store.Put(ctx, testKey, session.Record[payload]{
		Data: payload{AccountID: "a"}, CreatedAt: current, LastSeenAt: current,
		ExpiresAt: current.Add(time.Hour), IdleExpiresAt: current.Add(10 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	current = current.Add(11 * time.Minute)
	if _, err := store.Get(ctx, testKey); !errors.Is(err, session.ErrExpired) {
		t.Fatalf("idle expiry error = %v", err)
	}
}

func TestTouchNeverRevivesOrOutlivesAbsoluteExpiry(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	store, _ := testStore(t, func() time.Time { return current })
	ctx := context.Background()
	expiresAt := current.Add(time.Hour)
	if err := store.Put(ctx, testKey, session.Record[payload]{
		Data: payload{AccountID: "a"}, CreatedAt: current, LastSeenAt: current,
		ExpiresAt: expiresAt, IdleExpiresAt: current.Add(30 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	current = current.Add(10 * time.Minute)
	if err := store.Touch(ctx, testKey, current, current.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// A renewal past the absolute expiry is refused rather than clamped
	// silently by the caller.
	if err := store.Touch(ctx, testKey, current, expiresAt.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("renewal beyond absolute expiry = %v", err)
	}
	// A record nobody stored can never be created by Touch.
	other := strings.Repeat("a", 64)
	if err := store.Touch(ctx, other, current, current.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("touch of a missing record = %v", err)
	}
}

func TestDeleteIsIdempotentAndPruneRemovesExpired(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	store, db := testStore(t, func() time.Time { return current })
	ctx := context.Background()
	if err := store.Put(ctx, testKey, session.Record[payload]{
		Data: payload{AccountID: "a"}, CreatedAt: current, LastSeenAt: current,
		ExpiresAt: current.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, testKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, testKey); err != nil {
		t.Fatalf("second delete = %v", err)
	}

	if err := store.Put(ctx, testKey, session.Record[payload]{
		Data: payload{AccountID: "a"}, CreatedAt: current, LastSeenAt: current,
		ExpiresAt: current.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	current = current.Add(2 * time.Minute)
	removed, err := store.Prune(ctx, current, 10)
	if err != nil || removed != 1 {
		t.Fatalf("prune removed %d err = %v", removed, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM popcornwave_session`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rows after prune = %d", count)
	}
}

func TestStoreRejectsUnsafeInputs(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store, db := testStore(t, func() time.Time { return now })
	ctx := context.Background()

	for _, key := range []string{"", "short", strings.Repeat("z", 64), strings.Repeat("A", 64)} {
		if err := store.Delete(ctx, key); !errors.Is(err, session.ErrInvalidKey) {
			t.Fatalf("key %q error = %v", key, err)
		}
	}
	// A table name is interpolated into SQL, so it may only be an identifier.
	for _, table := range []string{"session; DROP TABLE users", "1session", "session-1", ""} {
		if table == "" {
			continue
		}
		if _, err := NewStore[payload](db, session.JSONCodec[payload]{}, Options{Table: table}); err == nil {
			t.Fatalf("table %q was accepted", table)
		}
	}
	if _, err := NewStore[payload](db, session.JSONCodec[payload]{}, Options{MaxPayloadBytes: -1}); err == nil {
		t.Fatal("negative payload bound was accepted")
	}
}

func TestStoreRejectsOversizedPayload(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore[payload](db, session.JSONCodec[payload]{}, Options{
		Now: func() time.Time { return now }, MaxPayloadBytes: 16,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), testKey, session.Record[payload]{
		Data: payload{AccountID: strings.Repeat("a", 128)}, CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(time.Hour),
	})
	if !errors.Is(err, session.ErrCodec) {
		t.Fatalf("oversized payload error = %v", err)
	}
}
