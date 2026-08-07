package pwruntime_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/database"
	_ "github.com/shibukawa/popcornwave/database/postgres"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

// TestNativeRuntimePath runs the request-time seams end to end on the native
// PostgreSQL connection: executor resolution, query diagnostics decoration,
// the transaction runner with a rolled-back savepoint, and the absence of a
// *sql.DB.
func TestNativeRuntimePath(t *testing.T) {
	dsn := os.Getenv("PW_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set PW_POSTGRES_TEST_DSN to run this test")
	}
	target, err := database.Resolve(dsn)
	if err != nil {
		t.Fatal(err)
	}
	setupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	native, err := target.OpenNative(setupCtx, database.PoolBounds{MaxOpenConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Close() })

	table := "pw_native_runtime"
	if _, err := native.ExecContext(setupCtx, "DROP TABLE IF EXISTS "+table); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = native.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+table)
	})
	if _, err := native.ExecContext(setupCtx, "CREATE TABLE "+table+" (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	set, err := pwruntime.NewConnectionSet("", []pwruntime.Connection{{
		Native: native,
		Driver: target.Dialect,
		Group:  "default",
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := pwruntime.WithResources(context.Background(), pwruntime.Resources{
		Connections: set,
		DBDriver:    target.Dialect,
		// Diagnostics on, so the statements below run through the
		// instrumented executor and its QueryRows dispatch.
		Query: &pwruntime.QueryDiagnostics{Level: pwruntime.LevelDebug},
	})

	if _, ok := pwruntime.DB(ctx); ok {
		t.Fatal("a native connection produced a *sql.DB")
	}
	if _, ok := pwruntime.ConnectionExecutor(ctx); !ok {
		t.Fatal("no connection executor on the native connection")
	}

	// The seam every generated statement passes through.
	executor, err := pwruntime.SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 1, "seam"); err != nil {
		t.Fatal(err)
	}
	countThrough := func(ctx context.Context) int {
		t.Helper()
		executor, err := pwruntime.SQLExecutor(ctx)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := sqlbind.Query(ctx, executor, "SELECT COUNT(*) FROM "+table)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			t.Fatalf("no count row: %v", rows.Err())
		}
		var total int
		if err := rows.Scan(&total); err != nil {
			t.Fatal(err)
		}
		return total
	}
	if total := countThrough(ctx); total != 1 {
		t.Fatalf("rows through the seam = %d, want 1", total)
	}

	// The transaction runner: the nested call rolls back through its
	// savepoint, the outer work commits, and statements inside resolve the
	// transaction executor.
	sentinel := errors.New("inner failure")
	err = pwruntime.Transaction(ctx, func(txCtx context.Context) error {
		executor, err := pwruntime.SQLExecutor(txCtx)
		if err != nil {
			return err
		}
		if _, err := executor.ExecContext(txCtx, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 2, "outer"); err != nil {
			return err
		}
		nested := pwruntime.Transaction(txCtx, func(innerCtx context.Context) error {
			executor, err := pwruntime.SQLExecutor(innerCtx)
			if err != nil {
				return err
			}
			if _, err := executor.ExecContext(innerCtx, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 3, "inner"); err != nil {
				return err
			}
			return sentinel
		})
		if !errors.Is(nested, sentinel) {
			return errors.New("nested transaction did not surface the inner failure")
		}
		if total := countThrough(txCtx); total != 2 {
			t.Fatalf("rows inside the transaction = %d, want 2", total)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total := countThrough(ctx); total != 2 {
		t.Fatalf("rows after commit = %d, want 2", total)
	}
}
