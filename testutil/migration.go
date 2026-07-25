package testutil

import (
	"context"
	"database/sql"
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
	script, err := cachedSnapshot(ctx, options)
	if err != nil {
		return err
	}
	return migrate.Replay(ctx, db, script)
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
