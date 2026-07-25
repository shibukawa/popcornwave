package pwruntime

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/shibukawa/tinygodriver/database/sqlite"
)

var errBoom = errors.New("boom")

func newTestDB(t *testing.T, driver string) (*sql.DB, context.Context) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	return db, WithResources(context.Background(), Resources{DB: db, DBDriver: driver})
}

func insert(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES (?)", name); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT name FROM items ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		result = append(result, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}
}

func TestNestedTransactionCommitsBothLevels(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	err := Transaction(ctx, func(ctx context.Context) error {
		insert(t, ctx, "outer")
		return Transaction(ctx, func(ctx context.Context) error {
			insert(t, ctx, "inner")
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, names(t, db), []string{"inner", "outer"})
}

func TestNestedFailureRollsBackOnlyInnerWork(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	err := Transaction(ctx, func(ctx context.Context) error {
		insert(t, ctx, "outer")
		nestedErr := Transaction(ctx, func(ctx context.Context) error {
			insert(t, ctx, "inner")
			return errBoom
		})
		if !errors.Is(nestedErr, errBoom) {
			t.Fatalf("nested error = %v, want %v", nestedErr, errBoom)
		}
		// The caller absorbs the inner failure and keeps committing.
		insert(t, ctx, "after")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, names(t, db), []string{"after", "outer"})
}

func TestNestedTransactionsNestThreeLevels(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	err := Transaction(ctx, func(ctx context.Context) error {
		insert(t, ctx, "level0")
		return Transaction(ctx, func(ctx context.Context) error {
			insert(t, ctx, "level1")
			nestedErr := Transaction(ctx, func(ctx context.Context) error {
				insert(t, ctx, "level2")
				return errBoom
			})
			if !errors.Is(nestedErr, errBoom) {
				t.Fatalf("nested error = %v, want %v", nestedErr, errBoom)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, names(t, db), []string{"level0", "level1"})
}

func TestNestedPanicUnwindsAndPropagates(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("panic did not propagate")
			}
		}()
		_ = Transaction(ctx, func(ctx context.Context) error {
			insert(t, ctx, "outer")
			return Transaction(ctx, func(ctx context.Context) error {
				insert(t, ctx, "inner")
				panic("inner")
			})
		})
	}()
	equal(t, names(t, db), nil)
}

func TestNestedFailsOnDriverWithoutSavepoints(t *testing.T) {
	db, ctx := newTestDB(t, "unknown-driver")
	err := Transaction(ctx, func(ctx context.Context) error {
		insert(t, ctx, "outer")
		return Transaction(ctx, func(ctx context.Context) error {
			insert(t, ctx, "inner")
			return nil
		})
	})
	if !errors.Is(err, ErrSavepointUnsupported) {
		t.Fatalf("error = %v, want %v", err, ErrSavepointUnsupported)
	}
	equal(t, names(t, db), nil)
}

func TestApplicationTransactionNestsIntoOwnerScope(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	scope := NewTransactionScope(db, "sqlite")
	if err := scope.Begin(ctx, nil); err != nil {
		t.Fatal(err)
	}
	owned := withScope(ctx, scope)

	if err := Transaction(owned, func(ctx context.Context) error {
		insert(t, ctx, "committed-by-application")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The application transaction released its savepoint but did not commit.
	equal(t, names(t, db), nil)

	if err := scope.Rollback(); err != nil {
		t.Fatal(err)
	}
	equal(t, names(t, db), nil)
}

func TestFailedScopeRefusesCommit(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	scope := NewTransactionScope(db, "sqlite")
	if err := scope.Begin(ctx, nil); err != nil {
		t.Fatal(err)
	}
	insert(t, withScope(ctx, scope), "poisoned")
	scope.markFailed()

	err := Transaction(withScope(ctx, scope), func(context.Context) error { return nil })
	if !errors.Is(err, ErrTransactionFailed) {
		t.Fatalf("nested error = %v, want %v", err, ErrTransactionFailed)
	}
	if err := scope.Commit(); !errors.Is(err, ErrTransactionFailed) {
		t.Fatalf("commit error = %v, want %v", err, ErrTransactionFailed)
	}
	equal(t, names(t, db), nil)
}

func TestSupportsSavepoint(t *testing.T) {
	for _, driver := range []string{"sqlite", "sqlite3", "postgres", "pgx", "mysql"} {
		if !SupportsSavepoint(driver) {
			t.Errorf("driver %q must support savepoints", driver)
		}
	}
	for _, driver := range []string{"", "oracle", "unknown-driver"} {
		if SupportsSavepoint(driver) {
			t.Errorf("driver %q must not claim savepoint support", driver)
		}
	}
}
