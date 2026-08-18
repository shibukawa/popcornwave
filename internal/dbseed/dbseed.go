// Package dbseed applies version-control-owned dataset files to the
// framework-owned database pool.
//
// It is the single place where Popcorn Web talks to github.com/shibukawa/dbtestify,
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
	"github.com/shibukawa/popcornweb/database"
	"github.com/shibukawa/tinybind-go/sqlbind"
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

// Executor is the statement target datasets are applied through: dbtestify's
// driver-agnostic execution interface since its v0.5.0. A database/sql handle
// enters through FromSQL, the framework executor seam through FromRuntime.
type Executor = dbtestify.Executor

// FromSQL adapts a *sql.DB, *sql.Tx, or *sql.Conn to Executor. A *sql.DB is
// remembered as a pool, so Seed can open its per-dataset transaction on it.
func FromSQL(handle dbtestify.SQLHandle) Executor {
	if db, ok := handle.(*sql.DB); ok {
		return sqlPoolExecutor{Executor: dbtestify.FromSQL(db), db: db}
	}
	return dbtestify.FromSQL(handle)
}

// FromRuntime adapts a framework executor — the pool-level executor of a
// connection or the executor of an open transaction scope — to Executor. A
// native executor queries through its QueryRows; a database/sql handle goes
// through FromSQL. A nil executor returns nil, which Apply and Assert report
// as "no database to seed".
func FromRuntime(executor sqlbind.SQLExecutor) Executor {
	switch typed := executor.(type) {
	case nil:
		return nil
	case sqlbind.RowsQuerier:
		return runtimeExecutor{exec: executor, rows: typed}
	case dbtestify.SQLHandle:
		return FromSQL(typed)
	default:
		// sqlbind.SQLExecutor includes QueryContext, so every executor that is
		// not a RowsQuerier is a SQLHandle; this arm is unreachable.
		return nil
	}
}

// sqlPoolExecutor keeps the *sql.DB visible through the Executor, because a
// pool-backed Seed runs each dataset in a transaction dbtestify opens itself.
type sqlPoolExecutor struct {
	Executor
	db *sql.DB
}

// runtimeExecutor bridges the sqlbind executor surface onto dbtestify's.
type runtimeExecutor struct {
	exec sqlbind.SQLExecutor
	rows sqlbind.RowsQuerier
}

// beginNativeTx opens a transaction when the wrapped executor is a native
// pool, which is what lets a pool-backed Apply keep its per-dataset
// transaction semantics off database/sql.
func (r runtimeExecutor) beginNativeTx(ctx context.Context) (database.NativeTx, bool, error) {
	pool, ok := r.exec.(database.NativeDB)
	if !ok {
		return nil, false, nil
	}
	tx, err := pool.BeginTx(ctx, database.NativeTxOptions{})
	if err != nil {
		return nil, true, err
	}
	return tx, true, nil
}

func (r runtimeExecutor) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := r.exec.ExecContext(ctx, query, args...)
	if err != nil {
		return dbtestify.UnknownAffected, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return dbtestify.UnknownAffected, nil
	}
	return affected, nil
}

func (r runtimeExecutor) Query(ctx context.Context, query string, args ...any) (dbtestify.Rows, error) {
	rows, err := r.rows.QueryRows(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

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
//
// A dataset's own _operation block selects the per-table operation. dbtestify
// reads operations from the option struct rather than from the parsed dataset,
// so the caller has to carry them across — its CLI and HTTP API do the same.
// Omitting that step silently clear-inserts every table.
func Apply(ctx context.Context, exec Executor, dialect Dialect, inTransaction bool, paths []string) error {
	// dbtestify opens its per-dataset transaction only on a *sql.DB, so a
	// native pool gets the equivalent here: one native transaction per
	// dataset, committed on success.
	if runtime, ok := exec.(runtimeExecutor); ok && !inTransaction {
		if handled, err := runtime.applyInNativeTx(ctx, dialect, paths); handled {
			return err
		}
	}
	connector, err := connector(exec, dialect, inTransaction)
	if err != nil {
		return err
	}
	for _, path := range paths {
		dataset, err := parse(path)
		if err != nil {
			return err
		}
		if err := dbtestify.Seed(ctx, connector, dataset, dbtestify.SeedOpt{
			Operations: dataset.Operation,
		}); err != nil {
			return fmt.Errorf("seed %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// applyInNativeTx mirrors dbtestify's pool behavior for a native pool. It
// reports false without error when the wrapped executor is no pool, in which
// case the ordinary path serves the call.
func (r runtimeExecutor) applyInNativeTx(ctx context.Context, dialect Dialect, paths []string) (bool, error) {
	for _, path := range paths {
		tx, native, err := r.beginNativeTx(ctx)
		if !native {
			return false, nil
		}
		if err != nil {
			return true, err
		}
		if err := seedOne(ctx, FromRuntime(tx), dialect, true, path); err != nil {
			_ = tx.Rollback(ctx)
			return true, err
		}
		if err := tx.Commit(ctx); err != nil {
			return true, fmt.Errorf("seed %s: %w", filepath.Base(path), err)
		}
	}
	return true, nil
}

// seedOne applies a single dataset through an executor already positioned —
// inside a transaction or on a pool — by the caller.
func seedOne(ctx context.Context, exec Executor, dialect Dialect, inTransaction bool, path string) error {
	connector, err := connector(exec, dialect, inTransaction)
	if err != nil {
		return err
	}
	dataset, err := parse(path)
	if err != nil {
		return err
	}
	if err := dbtestify.Seed(ctx, connector, dataset, dbtestify.SeedOpt{
		Operations: dataset.Operation,
	}); err != nil {
		return fmt.Errorf("seed %s: %w", filepath.Base(path), err)
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

// connector selects the dbtestify connector shape for exec. A *sql.DB pool
// keeps its pool-owning connector, which is what lets Seed open its own
// per-dataset transaction there.
func connector(exec Executor, dialect Dialect, inTransaction bool) (dbtestify.DBConnector, error) {
	if exec == nil {
		return nil, fmt.Errorf("no database to seed")
	}
	if pool, ok := exec.(sqlPoolExecutor); ok && !inTransaction {
		return dbtestify.NewDBConnectorFromDB(pool.db, dialect)
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
