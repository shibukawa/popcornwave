package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfigMigrationDefaults(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, `[project]
name = "fixture"
main = "./cmd/fixture"
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Migration.Dir != defaultMigrationDir {
		t.Fatalf("migration dir = %q, want %q", config.Migration.Dir, defaultMigrationDir)
	}
	if !config.Migration.Auto {
		t.Fatal("migration.auto should default to true for pw dev")
	}
}

func TestLoadProjectConfigMigrationOverrides(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, `[project]
name = "fixture"
main = "./cmd/fixture"

[migration]
dir = "db/migrations"
auto = false
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Migration.Dir != "db/migrations" || config.Migration.Auto {
		t.Fatalf("unexpected migration config: %#v", config.Migration)
	}
}

func TestLoadProjectConfigRejectsAbsoluteMigrationDir(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, `[project]
name = "fixture"
main = "./cmd/fixture"

[migration]
dir = "/etc/migrations"
`)
	if _, err := loadProjectConfig(root); err == nil {
		t.Fatal("absolute migration.dir was accepted")
	}
}

// TestMigrationWatchPathsIncludesPlainSQL guards the dev watch set. Migration
// files end in .sql rather than .pw.sql, so the default source walk skips them.
func TestMigrationWatchPathsIncludesPlainSQL(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "migrations")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "00001_init.sql"), "-- +goose Up\nSELECT 1;\n")
	writeTestFile(t, filepath.Join(directory, "README.md"), "notes\n")

	paths := migrationWatchPaths(root, migrationConfig{Dir: "migrations"})
	if len(paths) != 1 || filepath.Base(paths[0]) != "00001_init.sql" {
		t.Fatalf("watch paths = %v, want the migration only", paths)
	}

	state, err := snapshotWatchFiles(root, nil, configuredWatchPaths(root, nil, paths)...)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state[paths[0]]; !ok {
		t.Fatalf("migration file is not watched; watched %d file(s)", len(state))
	}
}

func TestMigrationWatchPathsToleratesMissingDirectory(t *testing.T) {
	if paths := migrationWatchPaths(t.TempDir(), migrationConfig{Dir: "migrations"}); len(paths) != 0 {
		t.Fatalf("watch paths = %v, want none", paths)
	}
}

// TestRunDevMigrationsSkips checks the two cases where the dev loop must not
// attempt a migration: the feature is off, or the project has no migrations.
func TestRunDevMigrationsSkips(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr strings.Builder

	disabled := projectConfig{Migration: migrationConfig{Dir: "migrations", Auto: false}}
	if err := runDevMigrations(t.Context(), root, disabled, &stdout, &stderr); err != nil {
		t.Fatalf("disabled migration reported an error: %v", err)
	}

	enabled := projectConfig{Migration: migrationConfig{Dir: "migrations", Auto: true}}
	if err := runDevMigrations(t.Context(), root, enabled, &stdout, &stderr); err != nil {
		t.Fatalf("missing migration directory reported an error: %v", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("skipped migration wrote output: %q %q", stdout.String(), stderr.String())
	}
}

// The developer loop walks the module, so a heavy dependency tree is trimmed
// with dev.watch.excludes rather than by narrowing what triggers a rebuild.
func TestSnapshotWatchFilesSkipsExcludedSubtrees(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"handlers", filepath.Join("web", "node_modules", "pkg")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "handlers", "home_handler.go"), "package handlers\n")
	writeTestFile(t, filepath.Join(root, "web", "node_modules", "pkg", "index.go"), "package pkg\n")

	state, err := snapshotWatchFiles(root, []string{"web/node_modules"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state[filepath.Join(root, "handlers", "home_handler.go")]; !ok {
		t.Fatal("an ordinary Go source must stay in the watch set")
	}
	if _, ok := state[filepath.Join(root, "web", "node_modules", "pkg", "index.go")]; ok {
		t.Fatal("an excluded subtree must not be watched")
	}
}
