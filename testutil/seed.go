package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornwave/internal/dbseed"
	"github.com/shibukawa/popcornwave/pw"
)

// WithSeed loads dataset files into the copied database after WithSchemaDir has
// applied the schema and before the HTTP server starts.
//
// Each name is a path relative to the seed directory; the .yaml extension may
// be omitted. Datasets are applied in the given order.
func WithSeed(files ...string) RunOption {
	return func(settings *runSettings) error {
		if len(files) == 0 {
			return fmt.Errorf("testutil: WithSeed requires at least one dataset")
		}
		for _, file := range files {
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("testutil: empty dataset name")
			}
		}
		settings.seedFiles = append(settings.seedFiles, files...)
		return nil
	}
}

// WithSeedDir overrides the dataset directory, which defaults to testdata/seed
// relative to the test package directory.
func WithSeedDir(directory string) RunOption {
	return func(settings *runSettings) error {
		if strings.TrimSpace(directory) == "" {
			return fmt.Errorf("testutil: empty seed directory")
		}
		settings.seedDir = directory
		return nil
	}
}

// Seed loads dataset files into the running server's database.
//
// Use it to reset state between phases of one test. A failure stops the test.
//
// It is unavailable under WithTransaction because it works on the pool, not on
// the test transaction. Pass the datasets to WithSeed instead, which loads them
// before the transaction opens.
func (server *Server) Seed(t TestingT, files ...string) {
	t.Helper()
	if len(files) == 0 {
		t.Fatalf("testutil: Seed requires at least one dataset")
		return
	}
	if server.transaction {
		t.Fatalf("testutil: Seed is unavailable under WithTransaction; use WithSeed to load datasets before the transaction opens")
		return
	}
	if err := applySeed(server.Config, server.DB, server.seedDir, files); err != nil {
		t.Fatalf("seed Popcorn Wave database: %v", err)
	}
}

// AssertDB compares the running server's database against expected datasets.
//
// A mismatch is reported through Errorf with a plain-text per-table diff and
// the test continues. Only committed state is visible, so a request whose
// transaction is still open has not been compared yet.
//
// It is unavailable under WithTransaction because it reads through the pool and
// would never observe writes made inside the test transaction.
func (server *Server) AssertDB(t TestingT, files ...string) {
	t.Helper()
	if len(files) == 0 {
		t.Fatalf("testutil: AssertDB requires at least one dataset")
		return
	}
	if server.transaction {
		t.Fatalf("testutil: AssertDB is unavailable under WithTransaction; the pool cannot observe writes inside the test transaction")
		return
	}
	dialect, paths, err := resolveSeed(server.Config, server.seedDir, files)
	if err != nil {
		t.Fatalf("assert Popcorn Wave database: %v", err)
		return
	}
	matched, report, err := dbseed.Assert(context.Background(), server.DB, dialect, paths)
	if err != nil {
		t.Fatalf("assert Popcorn Wave database: %v", err)
		return
	}
	if !matched {
		t.Errorf("Popcorn Wave database does not match:\n%s", report)
	}
}

func applySeed(config *Config, db *sql.DB, directory string, files []string) error {
	dialect, paths, err := resolveSeed(config, directory, files)
	if err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("configured RDB is disabled")
	}
	return dbseed.Apply(context.Background(), db, dialect, paths)
}

func resolveSeed(config *Config, directory string, files []string) (dbseed.Dialect, []string, error) {
	middleware := Get[pw.MiddlewareConfig](config)
	if !middleware.RDB.Enabled {
		return "", nil, fmt.Errorf("configured RDB is disabled")
	}
	dialect, err := dbseed.ResolveDialect(middleware.RDB.DSN)
	if err != nil {
		return "", nil, err
	}
	if directory == "" {
		directory = dbseed.DefaultDir
	}
	paths, err := dbseed.Resolve(directory, files)
	if err != nil {
		return "", nil, err
	}
	return dialect, paths, nil
}
