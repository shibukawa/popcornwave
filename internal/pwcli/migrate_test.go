package pwcli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwmigrate"
)

func TestParseMigrateArgs(t *testing.T) {
	options, err := parseMigrateArgs([]string{"up"})
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if options.action != "up" || options.dir != "" || options.dsn != "" {
		t.Fatalf("options = %+v", options)
	}

	options, err = parseMigrateArgs([]string{"up-to", "5", "--dir", "db/migrations", "--dsn=sqlite://app.db"})
	if err != nil {
		t.Fatalf("up-to: %v", err)
	}
	if options.version != 5 || options.dir != "db/migrations" || options.dsn != "sqlite://app.db" {
		t.Fatalf("options = %+v", options)
	}

	options, err = parseMigrateArgs([]string{"down-to", "0", "--yes", "--dry-run"})
	if err != nil {
		t.Fatalf("down-to: %v", err)
	}
	if !options.confirm || !options.dryRun || options.version != 0 {
		t.Fatalf("options = %+v", options)
	}

	options, err = parseMigrateArgs([]string{"create", "add_posts"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if options.name != "add_posts" {
		t.Fatalf("options = %+v", options)
	}
}

func TestParseMigrateArgsRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"sideways"},
		{"up-to"},
		{"up-to", "abc"},
		{"create"},
		{"up", "extra"},
		{"up", "--nope"},
		{"up", "--dir"},
	} {
		if _, err := parseMigrateArgs(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func statuses() []pwmigrate.Status {
	return []pwmigrate.Status{
		{Version: 1, Path: "00001_init.sql", Applied: true},
		{Version: 2, Path: "00002_posts.sql", Applied: true},
		{Version: 3, Path: "00003_index.sql"},
	}
}

// TestPlannedMigrationsOrdersRollbacksNewestFirst guards the direction of a
// rollback plan; reporting the oldest migration would tell an operator the
// opposite of what down actually does.
func TestPlannedMigrationsOrdersRollbacksNewestFirst(t *testing.T) {
	planned := plannedMigrations(statuses(), migrateOptions{action: "down"})
	if len(planned) != 1 || planned[0].Version != 2 {
		t.Fatalf("down plan = %+v, want only version 2", planned)
	}

	planned = plannedMigrations(statuses(), migrateOptions{action: "down-to", version: 0})
	if len(planned) != 2 || planned[0].Version != 2 || planned[1].Version != 1 {
		t.Fatalf("down-to plan = %+v, want 2 then 1", planned)
	}

	planned = plannedMigrations(statuses(), migrateOptions{action: "down-to", version: 1})
	if len(planned) != 1 || planned[0].Version != 2 {
		t.Fatalf("down-to 1 plan = %+v, want only version 2", planned)
	}
}

func TestPlannedMigrationsForUpwardActions(t *testing.T) {
	planned := plannedMigrations(statuses(), migrateOptions{action: "up"})
	if len(planned) != 1 || planned[0].Version != 3 {
		t.Fatalf("up plan = %+v, want only version 3", planned)
	}

	planned = plannedMigrations(statuses(), migrateOptions{action: "up-by-one"})
	if len(planned) != 1 || planned[0].Version != 3 {
		t.Fatalf("up-by-one plan = %+v", planned)
	}

	planned = plannedMigrations(statuses(), migrateOptions{action: "up-to", version: 2})
	if len(planned) != 0 {
		t.Fatalf("up-to 2 plan = %+v, want none pending at or below 2", planned)
	}
}

func TestRedactDSNHidesCredentials(t *testing.T) {
	dsn := "postgres://user:s3cret@host:5432/app"
	err := redactDSN(errors.New("connect "+dsn+" failed: s3cret rejected"), dsn)
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("credentials leaked: %v", err)
	}
	// The address is what makes the failure actionable: which server refused,
	// on which port, for which database.
	if !strings.Contains(err.Error(), "postgres://") || !strings.Contains(err.Error(), "host:5432/app") {
		t.Fatalf("redaction removed too much: %v", err)
	}
}

func TestRedactDSNKeepsCredentiallessDSN(t *testing.T) {
	dsn := "sqlite://app.db"
	err := redactDSN(errors.New("open "+dsn+": permission denied"), dsn)
	if !strings.Contains(err.Error(), "sqlite://app.db") {
		t.Fatalf("unexpected redaction: %v", err)
	}
}

// TestMigrateWithoutProjectRuns covers the delegated child invocation, which has
// an explicit directory and DSN and therefore no project to load.
func TestMigrateWithoutProjectRuns(t *testing.T) {
	directory := t.TempDir()
	migration := "-- +goose Up\nCREATE TABLE a (id INTEGER);\n\n-- +goose Down\nDROP TABLE a;\n"
	if err := os.WriteFile(filepath.Join(directory, "00001_init.sql"), []byte(migration), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	var stdout, stderr strings.Builder
	options := migrateOptions{
		action: "up",
		dir:    directory,
		dsn:    "sqlite://" + t.TempDir() + "/app.db",
	}
	missing := project{err: errors.New("popcornwave.toml not found")}
	if err := executeMigrate(t.Context(), missing, options, &stdout, &stderr); err != nil {
		t.Fatalf("migrate without a project: %v", err)
	}
	if !strings.Contains(stdout.String(), "version 0 -> 1") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestMigrateWithoutProjectNeedsExplicitInputs(t *testing.T) {
	missing := project{err: errors.New("popcornwave.toml not found")}
	var stdout, stderr strings.Builder
	if err := executeMigrate(t.Context(), missing, migrateOptions{action: "up"}, &stdout, &stderr); err == nil {
		t.Fatal("missing project and missing --dir were accepted")
	}
	if err := executeMigrate(t.Context(), missing,
		migrateOptions{action: "up", dir: t.TempDir()}, &stdout, &stderr); err == nil {
		t.Fatal("missing project and missing DSN were accepted")
	}
}
