package testutil

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pw"
)

func memberMigrationDir(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	migration := "-- +goose Up\nCREATE TABLE member (id INTEGER PRIMARY KEY, name TEXT NOT NULL);\n\n-- +goose Down\nDROP TABLE member;\n"
	if err := os.WriteFile(filepath.Join(directory, "00001_member.sql"), []byte(migration), 0o644); err != nil {
		t.Fatal(err)
	}
	return directory
}

func withMemberDatabase(config *Config) {
	Update[pw.ServerConfig](config, func(value *pw.ServerConfig) {
		value.Public.Enabled = false
	})
	Update[pw.MiddlewareConfig](config, func(value *pw.MiddlewareConfig) {
		value.RDB = pw.RDBConfig{
			Enabled:        true,
			DSN:            "sqlite://:memory:",
			ConnectTimeout: time.Second,
			MaxOpenConns:   1,
			MaxIdleConns:   1,
		}
	})
}

func memberNames(t *testing.T, server *Server) []string {
	t.Helper()
	rows, err := server.DB.QueryContext(t.Context(), "SELECT name FROM member ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}

func TestWithSeedLoadsDatasetBeforeRequests(t *testing.T) {
	server := TestRun(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		withMemberDatabase, WithMigrations(memberMigrationDir(t)), WithSeed("initial"))

	got := memberNames(t, server)
	want := []string{"Frank", "Grace"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("members = %v, want %v", got, want)
	}
}

func TestAssertDBMatchesAndReportsDiff(t *testing.T) {
	server := TestRun(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		withMemberDatabase, WithMigrations(memberMigrationDir(t)), WithSeed("initial.yaml"))

	// The seeded state matches its own dataset.
	server.AssertDB(t, "initial")

	if _, err := server.DB.ExecContext(t.Context(), "INSERT INTO member VALUES (3, 'Heidi')"); err != nil {
		t.Fatal(err)
	}
	server.AssertDB(t, "after_insert")

	// The stale dataset must now be reported, not silently accepted.
	recorder := &recordingT{TestingT: t}
	server.AssertDB(recorder, "initial")
	if len(recorder.errors) != 1 {
		t.Fatalf("errors = %v, want exactly one mismatch report", recorder.errors)
	}
	if !strings.Contains(recorder.errors[0], "Heidi") {
		t.Fatalf("diff does not mention the unexpected row: %s", recorder.errors[0])
	}
	if strings.Contains(recorder.errors[0], "\x1b[") {
		t.Fatalf("diff contains ANSI escapes: %q", recorder.errors[0])
	}
}

func TestSeedResetsStateMidTest(t *testing.T) {
	server := TestRun(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		withMemberDatabase, WithMigrations(memberMigrationDir(t)), WithSeed("after_insert"))

	if got := len(memberNames(t, server)); got != 3 {
		t.Fatalf("seeded rows = %d, want 3", got)
	}

	// clear-insert truncates first, so reseeding drops the extra row.
	server.Seed(t, "initial")
	if got := memberNames(t, server); len(got) != 2 {
		t.Fatalf("reseeded members = %v, want 2 rows", got)
	}
}

func TestWithSeedDirOverridesLocation(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "custom.yaml"), []byte(
		"member:\n- { id: 9, name: Ivan }\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	server := TestRun(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		withMemberDatabase, WithMigrations(memberMigrationDir(t)),
		WithSeedDir(directory), WithSeed("custom"))

	if got := memberNames(t, server); strings.Join(got, ",") != "Ivan" {
		t.Fatalf("members = %v, want [Ivan]", got)
	}
}

func TestSeedFailsWhenDatabaseDisabled(t *testing.T) {
	recorder := &recordingT{TestingT: t}
	server := &Server{Config: &Config{values: nil}, seedDir: "testdata/seed"}
	server.Seed(recorder, "initial")

	if !strings.Contains(recorder.failure, "RDB is disabled") {
		t.Fatalf("failure = %q, want a disabled-RDB report", recorder.failure)
	}
}

func TestSeedRejectsUnknownDataset(t *testing.T) {
	server := TestRun(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		withMemberDatabase, WithMigrations(memberMigrationDir(t)))

	recorder := &recordingT{TestingT: t}
	server.Seed(recorder, "missing")
	if !strings.Contains(recorder.failure, "missing.yaml") {
		t.Fatalf("failure = %q, want a missing-dataset report", recorder.failure)
	}
}

// TestSeedAndAssertRejectTransactionServer pins the documented limitation: both
// work on the pool, which cannot see writes inside the test transaction.
func TestSeedAndAssertRejectTransactionServer(t *testing.T) {
	server := &Server{Config: &Config{values: nil}, seedDir: "testdata/seed", transaction: true}

	seedRecorder := &recordingT{TestingT: t}
	server.Seed(seedRecorder, "initial")
	if !strings.Contains(seedRecorder.failure, "WithTransaction") {
		t.Fatalf("Seed failure = %q, want a WithTransaction report", seedRecorder.failure)
	}

	assertRecorder := &recordingT{TestingT: t}
	server.AssertDB(assertRecorder, "initial")
	if !strings.Contains(assertRecorder.failure, "WithTransaction") {
		t.Fatalf("AssertDB failure = %q, want a WithTransaction report", assertRecorder.failure)
	}
}
