// Package dbseed applies version-control-owned dataset files to the
// framework-owned database pool.
//
// It is the single place where Popcorn Wave talks to github.com/shibukawa/dbtestify,
// so the CLI seed command and the test helpers share one dataset contract.
package dbseed

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/dbtestify"
)

// DefaultDir is the dataset directory relative to the project root for the CLI
// and to the test package directory for tests.
const DefaultDir = "testdata/seed"

// Extension is the only dataset file extension.
const Extension = ".yaml"

// Resolve turns dataset names into file paths under directory.
//
// An empty names slice selects every dataset in the directory in lexical order.
// A name may omit the .yaml extension.
func Resolve(directory string, names []string) ([]string, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("empty seed directory")
	}
	if len(names) == 0 {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		var paths []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), Extension) {
				paths = append(paths, filepath.Join(directory, entry.Name()))
			}
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			return nil, fmt.Errorf("no %s files in %s", Extension, directory)
		}
		return paths, nil
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("empty dataset name")
		}
		if filepath.Ext(name) == "" {
			name += Extension
		}
		path := filepath.Join(directory, filepath.FromSlash(name))
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("dataset %s: %w", name, err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// Dialect is the SQL flavor a dataset is applied with.
type Dialect = dbtestify.Dialect

// Executor is the statement target datasets are applied through. Both *sql.DB
// and *sql.Tx satisfy it, so a test can seed inside its own transaction.
type Executor = dbtestify.Executor

// ResolveDialect derives the dialect from a driver://dsn configuration value.
func ResolveDialect(dsn string) (Dialect, error) {
	dialect, _, err := dbtestify.SplitSource(dsn)
	if err != nil {
		return "", fmt.Errorf("database DSN: %w", err)
	}
	return dialect, nil
}

// Apply loads each dataset through exec in the given order.
//
// exec stays owned by the caller. Pass inTransaction when exec is already
// inside a transaction, in which case dbtestify neither commits nor rolls back
// and the caller's rollback undoes the seeding. Otherwise each dataset is
// applied in its own transaction. The first failure stops the run.
func Apply(ctx context.Context, exec Executor, dialect Dialect, inTransaction bool, paths []string) error {
	connector, err := connector(exec, dialect, inTransaction)
	if err != nil {
		return err
	}
	for _, path := range paths {
		dataset, err := parse(path)
		if err != nil {
			return err
		}
		if err := dbtestify.Seed(ctx, connector, dataset, dbtestify.SeedOpt{}); err != nil {
			return fmt.Errorf("seed %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// Assert compares the database against each dataset and returns a plain-text
// diff for every mismatching table.
//
// An exec already inside a transaction observes that transaction's uncommitted
// writes; a pool does not.
//
// The boolean reports whether every dataset matched. A non-nil error means the
// comparison could not run at all.
func Assert(ctx context.Context, exec Executor, dialect Dialect, inTransaction bool, paths []string) (bool, string, error) {
	connector, err := connector(exec, dialect, inTransaction)
	if err != nil {
		return false, "", err
	}
	matched := true
	var report strings.Builder
	for _, path := range paths {
		dataset, err := parse(path)
		if err != nil {
			return false, "", err
		}
		var diff strings.Builder
		ok, _, err := dbtestify.Assert(ctx, connector, dataset, dbtestify.AssertOpt{
			DiffCallback: dbtestify.DumpDiffCallback(&diff, dbtestify.DiffFormat{
				ShowTableName: true,
				Quiet:         true,
				NoColor:       true,
			}),
		})
		if err != nil {
			return false, "", fmt.Errorf("assert %s: %w", filepath.Base(path), err)
		}
		if !ok {
			matched = false
			fmt.Fprintf(&report, "%s:\n%s", filepath.Base(path), diff.String())
		}
	}
	return matched, report.String(), nil
}

// connector selects the dbtestify connector shape for exec.
func connector(exec Executor, dialect Dialect, inTransaction bool) (dbtestify.DBConnector, error) {
	if exec == nil {
		return nil, fmt.Errorf("no database to seed")
	}
	if tx, ok := exec.(*sql.Tx); ok {
		return dbtestify.NewDBConnectorFromTx(tx, dialect)
	}
	return dbtestify.NewDBConnectorFromExecutor(exec, dialect, inTransaction)
}

func parse(path string) (*dbtestify.DataSet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	dataset, err := dbtestify.ParseYAML(file)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return dataset, nil
}
