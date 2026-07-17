package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/petitweb-go/contrib/authstate"
	dbsqlite "github.com/shibukawa/petitweb-go/contrib/database/sqlite"
)

type stringCodec struct{}

func (stringCodec) Encode(value string) ([]byte, error) {
	if value == "encode-error" {
		return nil, errors.New("secret encode error")
	}
	return append([]byte{1}, value...), nil
}

func (stringCodec) Decode(encoded []byte) (string, error) {
	if len(encoded) == 0 || encoded[0] != 1 || string(encoded[1:]) == "decode-error" {
		return "", errors.New("secret decode error")
	}
	return string(encoded[1:]), nil
}

func newTestStore(t *testing.T, now func() time.Time, namespace string) (*Store[string], func()) {
	t.Helper()
	db, err := dbsqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	store, err := NewStore[string](db, stringCodec{}, Options{Namespace: namespace, Now: now})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return store, func() { _ = db.Close() }
}

func TestStorePutTakeDuplicateAndExpiredReplacement(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	store, closeStore := newTestStore(t, func() time.Time { return now }, "test")
	defer closeStore()
	ctx := context.Background()
	if err := store.Put(ctx, "key", "value", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "key", "duplicate", now.Add(time.Minute)); !errors.Is(err, authstate.ErrAlreadyExists) {
		t.Fatalf("duplicate Put error = %v", err)
	}
	value, err := store.Take(ctx, "key")
	if err != nil || value != "value" {
		t.Fatalf("Take = (%q, %v)", value, err)
	}
	if _, err := store.Take(ctx, "key"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("second Take error = %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO petitweb_authstate VALUES(?, ?, ?, ?)`, "test", "expired", now.Add(-time.Second).UnixMilli(), []byte{1, 'x'}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "expired", "replacement", now.Add(time.Minute)); err != nil {
		t.Fatalf("expired replacement Put error = %v", err)
	}
	value, err = store.Take(ctx, "expired")
	if err != nil || value != "replacement" {
		t.Fatalf("replacement Take = (%q, %v)", value, err)
	}
}

func TestStoreConcurrentTakeReturnsValueOnce(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	store, closeStore := newTestStore(t, func() time.Time { return now }, "test")
	defer closeStore()
	if err := store.Put(context.Background(), "key", "value", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Take(context.Background(), "key")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, authstate.ErrNotFound) {
			t.Fatalf("Take error = %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successful Take count = %d, want 1", success)
	}
}

func TestStoreConsumesExpiredMalformedAndCodecFailure(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	store, closeStore := newTestStore(t, func() time.Time { return now }, "test")
	defer closeStore()
	insert := func(key string, expiresAt time.Time, payload []byte) {
		t.Helper()
		if _, err := store.db.Exec(`INSERT INTO petitweb_authstate VALUES(?, ?, ?, ?)`, "test", key, expiresAt.UnixMilli(), payload); err != nil {
			t.Fatal(err)
		}
	}
	insert("expired", now.Add(-time.Second), []byte{1, 'x'})
	if _, err := store.Take(context.Background(), "expired"); !errors.Is(err, authstate.ErrExpired) {
		t.Fatalf("expired Take error = %v", err)
	}
	insert("malformed", now.Add(time.Minute), []byte{})
	if _, err := store.Take(context.Background(), "malformed"); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("malformed Take error = %v", err)
	}
	insert("codec", now.Add(time.Minute), append([]byte{1}, "decode-error"...))
	if _, err := store.Take(context.Background(), "codec"); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("codec Take error = %v", err)
	}
	for _, key := range []string{"expired", "malformed", "codec"} {
		if _, err := store.Take(context.Background(), key); !errors.Is(err, authstate.ErrNotFound) {
			t.Fatalf("second Take(%s) error = %v", key, err)
		}
	}
}

func TestStorePruneIsBoundedAndNamespaced(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	store, closeStore := newTestStore(t, func() time.Time { return now }, "one")
	defer closeStore()
	for i := range 5 {
		namespace := "one"
		if i == 4 {
			namespace = "two"
		}
		if _, err := store.db.Exec(`INSERT INTO petitweb_authstate VALUES(?, ?, ?, ?)`, namespace, fmt.Sprintf("key-%d", i), now.Add(-time.Duration(i+1)*time.Second).UnixMilli(), []byte{1, 'x'}); err != nil {
			t.Fatal(err)
		}
	}
	affected, err := store.Prune(context.Background(), now, 2)
	if err != nil || affected != 2 {
		t.Fatalf("Prune = (%d, %v), want (2, nil)", affected, err)
	}
	var one, two int
	if err := store.db.QueryRow(`SELECT count(*) FROM petitweb_authstate WHERE namespace = 'one'`).Scan(&one); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT count(*) FROM petitweb_authstate WHERE namespace = 'two'`).Scan(&two); err != nil {
		t.Fatal(err)
	}
	if one != 2 || two != 1 {
		t.Fatalf("remaining rows = one:%d two:%d", one, two)
	}
}

func TestEnsureSchemaIsIdempotentAndRejectsIncompatibleTable(t *testing.T) {
	db, err := dbsqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	store, err := NewStore[string](db, stringCodec{}, Options{Namespace: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("second EnsureSchema error = %v", err)
	}
	badDB, err := dbsqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	badDB.SetMaxOpenConns(1)
	defer badDB.Close()
	if _, err := badDB.Exec(`CREATE TABLE petitweb_authstate(namespace TEXT)`); err != nil {
		t.Fatal(err)
	}
	badStore, err := NewStore[string](badDB, stringCodec{}, Options{Namespace: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := badStore.EnsureSchema(context.Background()); !errors.Is(err, authstate.ErrInvalidOptions) {
		t.Fatalf("incompatible EnsureSchema error = %v", err)
	}
}

func TestStoreRejectsInvalidInputsAndCancellation(t *testing.T) {
	db, err := dbsqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := NewStore[string](db, stringCodec{}, Options{}); !errors.Is(err, authstate.ErrInvalidOptions) {
		t.Fatalf("NewStore error = %v", err)
	}
	store, closeStore := newTestStore(t, time.Now, "test")
	defer closeStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(ctx, "key", "value", time.Now().Add(time.Minute)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Put error = %v", err)
	}
	if _, err := store.Prune(context.Background(), time.Now(), 0); !errors.Is(err, authstate.ErrInvalidOptions) {
		t.Fatalf("invalid Prune error = %v", err)
	}
	if err := store.Put(context.Background(), "codec", "encode-error", time.Now().Add(time.Minute)); !errors.Is(err, authstate.ErrCodec) || stringsContains(err.Error(), "secret") {
		t.Fatalf("codec Put error = %v", err)
	}
	_ = store.db.Close()
	if _, err := store.Take(context.Background(), "key"); !errors.Is(err, authstate.ErrUnavailable) {
		t.Fatalf("closed database Take error = %v", err)
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authstate.db")
	now := time.UnixMilli(1_800_000_000_000)
	open := func() (*Store[string], func()) {
		db, err := dbsqlite.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		store, err := NewStore[string](db, stringCodec{}, Options{Namespace: "test", Now: func() time.Time { return now }})
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := store.EnsureSchema(context.Background()); err != nil {
			db.Close()
			t.Fatal(err)
		}
		return store, func() { _ = db.Close() }
	}
	store, closeStore := open()
	if err := store.Put(context.Background(), "key", "value", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	closeStore()
	store, closeStore = open()
	defer closeStore()
	value, err := store.Take(context.Background(), "key")
	if err != nil || value != "value" {
		t.Fatalf("Take after reopen = (%q, %v)", value, err)
	}
}

func stringsContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
