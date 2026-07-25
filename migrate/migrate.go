// Package migrate applies versioned SQL migrations to a Popcorn Wave database.
//
// Importing this package opts the migration engine into the binary. A host Go
// build links goose directly; a TinyGo build runs the pw command as a child
// process instead, because goose is host-only. Both selections expose the same
// API and behavior.
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/internal/sqlscript"
)

// DefaultDir is the project-relative migration directory.
const DefaultDir = "migrations"

// DSNEnv carries a database DSN to a delegated pw child process. A DSN is never
// passed as a process argument.
const DSNEnv = "PW_MIGRATE_DSN"

// Applied describes one migration that ran.
type Applied struct {
	Version   int64
	Path      string
	Direction string
	Duration  time.Duration
}

// Result reports the version change produced by an action.
type Result struct {
	Previous int64
	Current  int64
	Applied  []Applied
}

// State is the recorded state of one migration source.
type State struct {
	Version   int64
	Path      string
	Applied   bool
	AppliedAt time.Time
}

type settings struct {
	dir  string
	fsys fs.FS
}

// Option configures a migration source.
type Option func(*settings) error

// WithDir reads migrations from a directory.
func WithDir(path string) Option {
	return func(target *settings) error {
		if strings.TrimSpace(path) == "" {
			return errors.New("migrate: empty migration directory")
		}
		target.dir = path
		return nil
	}
}

// WithFS reads migrations from an embedded or otherwise virtual tree.
func WithFS(fsys fs.FS) Option {
	return func(target *settings) error {
		if fsys == nil {
			return errors.New("migrate: nil migration filesystem")
		}
		target.fsys = fsys
		return nil
	}
}

func resolve(options []Option) (settings, error) {
	result := settings{dir: DefaultDir}
	for _, apply := range options {
		if apply == nil {
			continue
		}
		if err := apply(&result); err != nil {
			return settings{}, err
		}
	}
	return result, nil
}

// Replay executes a snapshot produced by Snapshot against an empty database.
// The script is plain SQL, so this works on every toolchain.
func Replay(ctx context.Context, db *sql.DB, script string) error {
	if db == nil {
		return errors.New("migrate: nil database")
	}
	if strings.TrimSpace(script) == "" {
		return nil
	}
	var enforced int
	restore := false
	// Foreign key enforcement cannot change inside a transaction, so it is
	// suspended before the load and restored afterwards.
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enforced); err == nil && enforced != 0 {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return fmt.Errorf("migrate: suspend foreign keys: %w", err)
		}
		restore = true
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate: replay snapshot: %w", err)
	}
	if err := sqlscript.Exec(ctx, tx, script); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migrate: replay snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate: replay snapshot: %w", err)
	}
	if restore {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
			return fmt.Errorf("migrate: restore foreign keys: %w", err)
		}
	}
	return nil
}

// SourceHash identifies a migration source tree so callers can cache a
// snapshot and invalidate it when any migration changes.
func SourceHash(options ...Option) (string, error) {
	resolved, err := resolve(options)
	if err != nil {
		return "", err
	}
	fsys := resolved.fsys
	if fsys == nil {
		fsys = os.DirFS(resolved.dir)
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", fmt.Errorf("migrate: read migrations: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", fmt.Errorf("migrate: read %s: %w", name, err)
		}
		fmt.Fprintf(digest, "%s\x00%d\x00", name, len(body))
		digest.Write(body)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
