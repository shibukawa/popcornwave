//go:build !tinygo

package sessionstore_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/database"
	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/popcornweb/sessionstore"

	_ "github.com/shibukawa/popcornweb/database/mysql"
	_ "github.com/shibukawa/popcornweb/database/postgres"
	_ "github.com/shibukawa/popcornweb/database/sqlite"
	_ "github.com/shibukawa/popcornweb/sessionstore/mysql"
	_ "github.com/shibukawa/popcornweb/sessionstore/postgres"
	_ "github.com/shibukawa/popcornweb/sessionstore/sqlite"
)

// The server engines need a live server, so their runs are opt-in. The same
// DSN variables the database package uses select them:
//
//	PW_POSTGRES_TEST_DSN='postgres://pw:pw@127.0.0.1:55432/pw?sslmode=disable' \
//	PW_MYSQL_TEST_DSN='mysql://pw:pw@tcp(127.0.0.1:53306)/pw' \
//	    go test ./sessionstore/
//
// scripts/test-session-engines.sh starts both and runs this file.
const (
	postgresDSNEnv = "PW_POSTGRES_TEST_DSN"
	mysqlDSNEnv    = "PW_MYSQL_TEST_DSN"
)

// TestEngineContract runs one suite against every engine, which is what makes
// them interchangeable rather than three separately plausible stores. SQLite
// needs no server and always runs; the others join when configured.
func TestEngineContract(t *testing.T) {
	for _, engine := range []struct{ dialect, dsn string }{
		{dialect: "sqlite", dsn: "sqlite://" + filepath.Join(t.TempDir(), "contract.db")},
		{dialect: "postgres", dsn: os.Getenv(postgresDSNEnv)},
		{dialect: "mysql", dsn: os.Getenv(mysqlDSNEnv)},
	} {
		t.Run(engine.dialect, func(t *testing.T) {
			if engine.dsn == "" {
				t.Skipf("set %s to run this engine", engine.dialect)
			}
			db := openEngine(t, engine.dsn)
			store, err := sessionstore.NewStore(db, sessionstore.Options{
				Dialect: engine.dialect,
				Table:   "popcornweb_session_contract",
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			// A fresh database has no table, which is what a project that
			// skipped its migration looks like.
			if err := store.VerifySchema(ctx); !errors.Is(err, sessionstore.ErrSchemaMissing) {
				t.Fatalf("VerifySchema before the migration = %v", err)
			}
			if err := store.EnsureSchema(ctx); err != nil {
				t.Fatalf("EnsureSchema: %v", err)
			}
			t.Cleanup(func() {
				_, _ = db.ExecContext(context.Background(), `DROP TABLE popcornweb_session_contract`)
			})
			if err := store.VerifySchema(ctx); err != nil {
				t.Fatalf("VerifySchema after the migration: %v", err)
			}

			now := time.Now().Truncate(time.Millisecond)
			record := session.RawRecord{
				Payload:         []byte(`{"account_id":"account-1"}`),
				CreatedAt:       now,
				AuthenticatedAt: now,
				LastSeenAt:      now,
				ExpiresAt:       now.Add(time.Hour),
				IdleExpiresAt:   now.Add(30 * time.Minute),
				Method:          "oidc",
				Version:         2,
			}
			key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			if err := store.Put(ctx, key, record); err != nil {
				t.Fatalf("Put: %v", err)
			}
			loaded, err := store.Get(ctx, key)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if string(loaded.Payload) != string(record.Payload) || loaded.Method != "oidc" || loaded.Version != 2 {
				t.Fatalf("record = %#v", loaded)
			}
			if !loaded.ExpiresAt.Equal(record.ExpiresAt) || !loaded.IdleExpiresAt.Equal(record.IdleExpiresAt) {
				t.Fatalf("timestamps = %v, %v", loaded.ExpiresAt, loaded.IdleExpiresAt)
			}

			// A second Put replaces the row rather than failing on the key.
			record.Method = "passkey"
			if err := store.Put(ctx, key, record); err != nil {
				t.Fatalf("replacing Put: %v", err)
			}
			if loaded, err = store.Get(ctx, key); err != nil || loaded.Method != "passkey" {
				t.Fatalf("replaced record = (%#v, %v)", loaded, err)
			}

			renewed := now.Add(45 * time.Minute)
			if err := store.Touch(ctx, key, now.Add(time.Minute), renewed); err != nil {
				t.Fatalf("Touch: %v", err)
			}
			if loaded, err = store.Get(ctx, key); err != nil || !loaded.IdleExpiresAt.Equal(renewed) {
				t.Fatalf("renewed record = (%#v, %v)", loaded, err)
			}
			// A renewal past the absolute expiry is refused by every engine.
			if err := store.Touch(ctx, key, now, record.ExpiresAt.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
				t.Fatalf("overextending Touch = %v", err)
			}

			// An expired record is swept, and a live one is left alone.
			expiredKey := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
			expired := record
			expired.ExpiresAt = now.Add(-time.Hour)
			expired.IdleExpiresAt = time.Time{}
			if err := store.Put(ctx, expiredKey, expired); err != nil {
				t.Fatalf("Put expired: %v", err)
			}
			swept, err := store.Prune(ctx, now, 16)
			if err != nil {
				t.Fatalf("Prune: %v", err)
			}
			if swept != 1 {
				t.Fatalf("Prune removed %d records, want 1", swept)
			}
			if _, err := store.Get(ctx, key); err != nil {
				t.Fatalf("Prune removed a live record: %v", err)
			}

			if err := store.Delete(ctx, key); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if _, err := store.Get(ctx, key); !errors.Is(err, session.ErrNotFound) {
				t.Fatalf("Get after Delete = %v", err)
			}
			// Delete is idempotent, and a renewal never recreates the row.
			if err := store.Delete(ctx, key); err != nil {
				t.Fatalf("second Delete: %v", err)
			}
			if err := store.Touch(ctx, key, now, now.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
				t.Fatalf("Touch after Delete = %v", err)
			}
		})
	}
}

func openEngine(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	target, err := database.Resolve(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db, err := target.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	return db
}
