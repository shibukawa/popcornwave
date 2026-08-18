package dbseed_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/database"
	_ "github.com/shibukawa/popcornweb/database/postgres"
	"github.com/shibukawa/popcornweb/internal/dbseed"
)

// TestNativeSeedAndAssert runs the dataset contract on the native PostgreSQL
// executor: a pool-level Apply commits each dataset in its own native
// transaction, and an in-transaction Apply and Assert observe uncommitted
// state and disappear with the rollback — which is exactly what
// testutil.WithTransaction does with them.
func TestNativeSeedAndAssert(t *testing.T) {
	dsn := os.Getenv("PW_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set PW_POSTGRES_TEST_DSN to run this test")
	}
	target, err := database.Resolve(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	native, err := target.OpenNative(ctx, database.PoolBounds{MaxOpenConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Close() })

	table := "pw_dbseed_native"
	if _, err := native.ExecContext(ctx, "DROP TABLE IF EXISTS "+table); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = native.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+table)
	})
	if _, err := native.ExecContext(ctx, "CREATE TABLE "+table+" (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	dialect, err := dbseed.ResolveDialect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	dataset := func(name, content string) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	initial := dataset("initial.yaml", table+":\n- { id: 1, name: Frank }\n- { id: 2, name: Grace }\n")

	countRows := func() int {
		t.Helper()
		rows, err := native.QueryRows(ctx, "SELECT COUNT(*) FROM "+table)
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

	// Pool-level Apply: dbseed opens and commits a native transaction itself.
	if err := dbseed.Apply(ctx, dbseed.FromRuntime(native), dialect, false, []string{initial}); err != nil {
		t.Fatal(err)
	}
	if total := countRows(); total != 2 {
		t.Fatalf("rows after pool seed = %d, want 2", total)
	}

	// In-transaction Apply and Assert: seeded rows are visible inside the
	// transaction, the assertion reads them there, and the rollback removes
	// them.
	tx, err := native.BeginTx(ctx, database.NativeTxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	inserted := dataset("inserted.yaml", "_operation:\n  "+table+": insert\n\n"+table+":\n- { id: 3, name: Heidi }\n")
	expected := dataset("expected.yaml", table+":\n- { id: 1, name: Frank }\n- { id: 2, name: Grace }\n- { id: 3, name: Heidi }\n")
	if err := dbseed.Apply(ctx, dbseed.FromRuntime(tx), dialect, true, []string{inserted}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	matched, report, err := dbseed.Assert(ctx, dbseed.FromRuntime(tx), dialect, true, []string{expected})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if !matched {
		_ = tx.Rollback(ctx)
		t.Fatalf("assert inside the transaction did not match:\n%s", report)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if total := countRows(); total != 2 {
		t.Fatalf("rows after rollback = %d, want 2", total)
	}

	// A mismatching assertion reports the difference rather than erroring.
	missing := dataset("missing.yaml", table+":\n- { id: 1, name: Frank }\n")
	matched, report, err = dbseed.Assert(ctx, dbseed.FromRuntime(native), dialect, false, []string{missing})
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("assert matched a dataset that is missing a row")
	}
	if !strings.Contains(report, table) {
		t.Fatalf("diff does not name the table:\n%s", report)
	}
}
