// Package pwmigrate applies versioned SQL migrations with goose.
//
// The package is host-only; goose does not compile with TinyGo. A TinyGo build
// reaches the same behavior by running the pw command as a child process.
package pwmigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

// DefaultDir is the project-relative migration directory.
const DefaultDir = "migrations"

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

// Status is the recorded state of one migration source.
type Status struct {
	Version   int64
	Path      string
	Applied   bool
	AppliedAt time.Time
}

// Target is an opened migration destination.
type Target struct {
	DB      *sql.DB
	Dialect goose.Dialect
	ownsDB  bool
}

// Close releases a connection opened by Open.
func (target *Target) Close() error {
	if target == nil || !target.ownsDB || target.DB == nil {
		return nil
	}
	return target.DB.Close()
}

// ParseDSN splits the framework driver://dsn syntax and selects a goose dialect.
func ParseDSN(configured string) (driver, dataSource string, dialect goose.Dialect, err error) {
	configured = strings.TrimSpace(configured)
	driver, remainder, ok := strings.Cut(configured, "://")
	if !ok || driver == "" || remainder == "" {
		return "", "", "", errors.New("dsn must use driver://dsn syntax")
	}
	switch driver {
	case "sqlite", "sqlite3":
		return driver, remainder, goose.DialectSQLite3, nil
	case "postgres", "postgresql":
		return driver, configured, goose.DialectPostgres, nil
	case "mysql":
		return driver, configured, goose.DialectMySQL, nil
	}
	return "", "", "", fmt.Errorf("unsupported migration driver %q", driver)
}

// Open connects to the configured database for a migration action.
func Open(dsn string) (*Target, error) {
	driver, dataSource, dialect, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return &Target{DB: db, Dialect: dialect, ownsDB: true}, nil
}

// AttachSQLite wraps a connection the caller already owns as a migration
// target. The caller keeps responsibility for closing it.
func AttachSQLite(db *sql.DB) *Target {
	return &Target{DB: db, Dialect: goose.DialectSQLite3}
}

// Sources reads migrations from a directory.
func Sources(dir string) (fs.FS, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("migration directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("migration directory: %s is not a directory", dir)
	}
	return os.DirFS(dir), nil
}

func newProvider(target *Target, sources fs.FS) (*goose.Provider, error) {
	if target == nil || target.DB == nil {
		return nil, errors.New("migration target is not open")
	}
	provider, err := goose.NewProvider(target.Dialect, target.DB, sources,
		goose.WithDisableGlobalRegistry(true),
		goose.WithLogger(goose.NopLogger()),
	)
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return provider, nil
}

// Action names one migration operation.
type Action string

// Supported migration actions.
const (
	ActionUp      Action = "up"
	ActionUpByOne Action = "up-by-one"
	ActionUpTo    Action = "up-to"
	ActionDown    Action = "down"
	ActionDownTo  Action = "down-to"
)

// Apply runs one action and reports the resulting version change.
func Apply(ctx context.Context, target *Target, sources fs.FS, action Action, version int64) (Result, error) {
	provider, err := newProvider(target, sources)
	if err != nil {
		return Result{}, err
	}
	previous, err := provider.GetDBVersion(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read applied version: %w", err)
	}
	var results []*goose.MigrationResult
	var single *goose.MigrationResult
	switch action {
	case ActionUp:
		results, err = provider.Up(ctx)
	case ActionUpByOne:
		single, err = provider.UpByOne(ctx)
		if errors.Is(err, goose.ErrNoNextVersion) {
			single, err = nil, nil
		}
	case ActionUpTo:
		results, err = provider.UpTo(ctx, version)
	case ActionDown:
		single, err = provider.Down(ctx)
		if errors.Is(err, goose.ErrNoNextVersion) {
			single, err = nil, nil
		}
	case ActionDownTo:
		results, err = provider.DownTo(ctx, version)
	default:
		return Result{}, fmt.Errorf("unknown migration action %q", action)
	}
	if single != nil {
		results = append(results, single)
	}
	result := Result{Previous: previous, Current: previous, Applied: appliedFrom(results)}
	current, versionErr := provider.GetDBVersion(ctx)
	if versionErr == nil {
		result.Current = current
	}
	if err != nil {
		return result, err
	}
	if versionErr != nil {
		return result, fmt.Errorf("read applied version: %w", versionErr)
	}
	return result, nil
}

func appliedFrom(results []*goose.MigrationResult) []Applied {
	applied := make([]Applied, 0, len(results))
	for _, item := range results {
		if item == nil || item.Source == nil {
			continue
		}
		applied = append(applied, Applied{
			Version:   item.Source.Version,
			Path:      item.Source.Path,
			Direction: item.Direction,
			Duration:  item.Duration,
		})
	}
	return applied
}

// Statuses reports every known migration and whether it is applied.
func Statuses(ctx context.Context, target *Target, sources fs.FS) ([]Status, error) {
	provider, err := newProvider(target, sources)
	if err != nil {
		return nil, err
	}
	reported, err := provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("read migration status: %w", err)
	}
	statuses := make([]Status, 0, len(reported))
	for _, item := range reported {
		if item == nil || item.Source == nil {
			continue
		}
		statuses = append(statuses, Status{
			Version:   item.Source.Version,
			Path:      item.Source.Path,
			Applied:   item.State == goose.StateApplied,
			AppliedAt: item.AppliedAt,
		})
	}
	return statuses, nil
}

// Version returns the current applied version.
func Version(ctx context.Context, target *Target, sources fs.FS) (int64, error) {
	provider, err := newProvider(target, sources)
	if err != nil {
		return 0, err
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("read applied version: %w", err)
	}
	return version, nil
}

// Pending lists migration versions that Up would apply.
func Pending(ctx context.Context, target *Target, sources fs.FS) ([]Status, error) {
	statuses, err := Statuses(ctx, target, sources)
	if err != nil {
		return nil, err
	}
	pending := make([]Status, 0, len(statuses))
	for _, status := range statuses {
		if !status.Applied {
			pending = append(pending, status)
		}
	}
	return pending, nil
}

// Validate parses and orders migration sources without touching the configured
// database. A throwaway in-memory database satisfies the provider, which
// requires a connection even when no statement runs.
func Validate(sources fs.FS) ([]Status, error) {
	scratch, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("validate migrations: %w", err)
	}
	defer scratch.Close()
	provider, err := newProvider(&Target{DB: scratch, Dialect: goose.DialectSQLite3}, sources)
	if err != nil {
		return nil, err
	}
	listed := provider.ListSources()
	statuses := make([]Status, 0, len(listed))
	for _, source := range listed {
		if source == nil {
			continue
		}
		if source.Type != goose.TypeSQL {
			return nil, fmt.Errorf("%s: only SQL migrations are supported", source.Path)
		}
		statuses = append(statuses, Status{Version: source.Version, Path: source.Path})
	}
	return statuses, nil
}

const migrationTemplate = `-- +goose Up
-- +goose StatementBegin
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
`

// Create writes a new empty annotated migration and returns its path.
func Create(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("migration name is required")
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '_', char == '-':
		default:
			return "", fmt.Errorf("migration name %q may use letters, digits, underscore, and hyphen only", name)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("migration directory: %w", err)
	}
	next, err := nextVersion(dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%05d_%s.sql", next, name))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create migration: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(migrationTemplate); err != nil {
		return "", fmt.Errorf("write migration: %w", err)
	}
	return path, nil
}

func nextVersion(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("migration directory: %w", err)
	}
	var highest int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			continue
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			continue
		}
		if version > highest {
			highest = version
		}
	}
	return highest + 1, nil
}

// SourceFiles lists migration file names in version order.
func SourceFiles(sources fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(sources, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}
