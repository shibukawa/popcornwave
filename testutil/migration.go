package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/shibukawa/popcornwave/migrate"
)

// snapshotCache keeps one snapshot per migration source tree for the lifetime of
// the test binary. Producing a snapshot is the expensive step, especially on the
// TinyGo path where it runs a pw child process, so every TestRun in a package
// pays it at most once.
var snapshotCache = struct {
	sync.Mutex
	scripts map[string]string
}{scripts: make(map[string]string)}

func installMigrations(ctx context.Context, db *sql.DB, options []migrate.Option) error {
	installed, err := alreadyInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		// Tests may deliberately share one database file, so the first TestRun
		// installs the schema and later ones reuse it.
		return nil
	}
	script, err := cachedSnapshot(ctx, options)
	if err != nil {
		return err
	}
	return migrate.Replay(ctx, db, script)
}

// alreadyInstalled reports whether this database carries applied migration
// versions from an earlier TestRun.
func alreadyInstalled(ctx context.Context, db *sql.DB) (bool, error) {
	var present int
	err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&present)
	if err != nil {
		return false, fmt.Errorf("testutil: inspect migration state: %w", err)
	}
	if present == 0 {
		return false, nil
	}
	var applied int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM goose_db_version").Scan(&applied); err != nil {
		return false, fmt.Errorf("testutil: read migration state: %w", err)
	}
	return applied > 0, nil
}

func cachedSnapshot(ctx context.Context, options []migrate.Option) (string, error) {
	key, err := migrate.SourceHash(options...)
	if err != nil {
		return "", err
	}
	snapshotCache.Lock()
	defer snapshotCache.Unlock()
	if script, ok := snapshotCache.scripts[key]; ok {
		return script, nil
	}
	script, err := migrate.Snapshot(ctx, options...)
	if err != nil {
		return "", err
	}
	snapshotCache.scripts[key] = script
	return script, nil
}
