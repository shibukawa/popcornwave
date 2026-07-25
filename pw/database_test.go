package pw

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/shibukawa/tinygodriver/database/sqlite"
)

func TestDatabaseTarget(t *testing.T) {
	driver, dsn, err := databaseTarget("sqlite://app.db")
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "app.db" {
		t.Fatalf("target = %q, %q", driver, dsn)
	}
	if _, _, err := databaseTarget("app.db"); err == nil {
		t.Fatal("DSN without scheme was accepted")
	}
}

func TestInitializeSchemaAppliesSQLFilesInOrder(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	configState.Lock()
	previous := configState.db
	configState.db = db
	configState.Unlock()
	t.Cleanup(func() {
		configState.Lock()
		configState.db = previous
		configState.Unlock()
	})

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "001_table.sql"), []byte(
		"CREATE TABLE counter (value INTEGER NOT NULL);",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "002_seed.sql"), []byte(
		"INSERT INTO counter (value) VALUES (1);",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initializeSchema(directory); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT value FROM counter").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
}
