package sessionstore_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/pw"
	"github.com/shibukawa/popcornweb/sessionstore"
	sessionsqlite "github.com/shibukawa/popcornweb/sessionstore/sqlite"

	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestImportingThePackageRegistersTheBackend(t *testing.T) {
	// This package's init is the whole opt-in: an application blank-imports it
	// and session.backend = "rdb" resolves.
	registered := false
	for _, name := range pw.SessionBackends() {
		registered = registered || name == pw.SessionBackendRDB
	}
	if !registered {
		t.Fatalf("registered backends = %v", pw.SessionBackends())
	}

	db := testDB(t)
	store, err := sessionstore.NewStore(db, sessionstore.Options{Dialect: sessionsqlite.Dialect})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(t.Context()); err != nil {
		t.Fatal(err)
	}
	backend, err := pw.OpenSessionBackend(t.Context(), pw.SessionConfig{
		Backend: pw.SessionBackendRDB,
		RDB:     pw.SessionRDBConfig{Source: "middleware", Table: sessionstore.DefaultTable},
	}, pw.SessionResources{DB: db, DBDriver: "sqlite"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if backend.Store == nil {
		t.Fatal("no store")
	}
	// The pool is the host's, so this backend closes nothing; its table needs
	// a sweep, so it hands one back.
	if backend.Close != nil {
		t.Fatal("backend claimed ownership of a lent pool")
	}
	if backend.Prune == nil {
		t.Fatal("backend brought no expiry sweep for its table")
	}
	if _, err := backend.Prune(t.Context(), time.Now(), 16); err != nil {
		t.Fatalf("prune: %v", err)
	}
}

func TestBackendRefusesToStartWithoutItsDependencies(t *testing.T) {
	open := func(config pw.SessionConfig, resources pw.SessionResources) error {
		_, err := pw.OpenSessionBackend(context.Background(), config, resources)
		return err
	}
	base := pw.SessionConfig{Backend: pw.SessionBackendRDB, RDB: pw.SessionRDBConfig{Source: "middleware"}}

	// No database at all.
	if err := open(base, pw.SessionResources{}); err == nil ||
		!strings.Contains(err.Error(), "middleware.rdb.enabled") {
		t.Fatalf("missing database error = %v", err)
	}
	// A table nobody migrated, reported with the migration to apply.
	if err := open(base, pw.SessionResources{DB: testDB(t), DBDriver: "sqlite"}); err == nil ||
		!strings.Contains(err.Error(), sessionstore.MigrationName) {
		t.Fatalf("missing schema error = %v", err)
	}
	// A source this plugin does not implement is its own error, not the host's.
	dedicated := base
	dedicated.RDB.Source = "dedicated"
	if err := open(dedicated, pw.SessionResources{DB: testDB(t), DBDriver: "sqlite"}); err == nil ||
		!strings.Contains(err.Error(), "session.rdb.source") {
		t.Fatalf("dedicated source error = %v", err)
	}
}
