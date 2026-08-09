package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/database"
	"github.com/shibukawa/popcornwave/database/postgres"
	_ "github.com/shibukawa/popcornwave/database/sqlite"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/database/pgx"
)

func TestWithConnRejectsNilCallback(t *testing.T) {
	err := postgres.WithConn(context.Background(), nil)
	if err == nil {
		t.Fatal("a nil callback was accepted")
	}
	if !strings.Contains(err.Error(), "callback") {
		t.Errorf("error = %q, want it to name the callback", err)
	}
}

// TestWithConnWithoutAConnection covers a handler calling WithConn on a
// context that carries no framework database at all, which must report that
// rather than panic on a nil executor.
func TestWithConnWithoutAConnection(t *testing.T) {
	err := postgres.WithConn(context.Background(), func(*pgx.Conn) error {
		t.Error("the callback ran without a connection")
		return nil
	})
	if err == nil {
		t.Fatal("a context with no connection was accepted")
	}
}

// TestWithConnRejectsForeignEngine proves the accessor names the engine it
// found instead of failing obscurely, because the likely mistake is
// PostgreSQL-only code running against a group configured for another
// database.
func TestWithConnRejectsForeignEngine(t *testing.T) {
	target, err := database.Resolve("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	db, err := target.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	set, err := pwruntime.NewConnectionSet("", []pwruntime.Connection{{
		DB:     db,
		Driver: target.Dialect,
		Group:  "default",
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := pwruntime.WithResources(context.Background(), pwruntime.Resources{
		Connections: set,
		DBDriver:    target.Dialect,
	})

	err = postgres.WithConn(ctx, func(*pgx.Conn) error {
		t.Error("the callback ran on a SQLite connection")
		return nil
	})
	if err == nil {
		t.Fatal("a SQLite group was accepted")
	}
	if !strings.Contains(err.Error(), target.Dialect) {
		t.Errorf("error = %q, want it to name %q", err, target.Dialect)
	}
}

// TestWithConnOnNativePostgres exercises the accessor against a real server:
// a batch on the pool, a batch inside pw.Transaction rolling back with it, and
// connection release on a pool of one.
//
// Start a server with:
//
//	docker run -d --name pgtest -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=pw \
//	    -e POSTGRES_DB=pw -p 55432:5432 postgres:17
func TestWithConnOnNativePostgres(t *testing.T) {
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
	// One connection, so a callback that failed to release would make the
	// next acquisition time out instead of quietly succeeding on a second.
	native, err := target.OpenNative(setupCtx, database.PoolBounds{MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Close() })

	table := "pw_withconn"
	if _, err := native.ExecContext(setupCtx, "DROP TABLE IF EXISTS "+table); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = native.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+table)
	})
	if _, err := native.ExecContext(setupCtx,
		"CREATE TABLE "+table+" (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
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
	})

	insertBatch := func(ctx context.Context, conn *pgx.Conn, ids ...int) error {
		batch := &pgx.Batch{}
		for _, id := range ids {
			batch.Queue("INSERT INTO "+table+" (id, name) VALUES ($1, $2)", id, "batched")
		}
		results := conn.SendBatch(ctx, batch)
		for range ids {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return err
			}
		}
		return results.Close()
	}
	count := func(ctx context.Context) int {
		t.Helper()
		var total int
		if err := postgres.WithConn(ctx, func(conn *pgx.Conn) error {
			return conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&total)
		}); err != nil {
			t.Fatal(err)
		}
		return total
	}

	// On the pool, outside any transaction.
	if err := postgres.WithConn(ctx, func(conn *pgx.Conn) error {
		return insertBatch(ctx, conn, 1, 2, 3)
	}); err != nil {
		t.Fatal(err)
	}
	// A second call, which only succeeds if the first released its connection.
	releaseCtx, releaseCancel := context.WithTimeout(ctx, 10*time.Second)
	defer releaseCancel()
	if total := count(releaseCtx); total != 3 {
		t.Fatalf("rows after the batch = %d, want 3", total)
	}

	// Inside a transaction the callback runs on the transaction's own
	// connection, so its work rolls back with it.
	sentinel := errors.New("rolled back")
	err = pwruntime.Transaction(ctx, func(txCtx context.Context) error {
		if err := postgres.WithConn(txCtx, func(conn *pgx.Conn) error {
			return insertBatch(txCtx, conn, 4, 5)
		}); err != nil {
			return err
		}
		if total := count(txCtx); total != 5 {
			t.Errorf("rows inside the transaction = %d, want 5", total)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Transaction error = %v, want the sentinel", err)
	}
	if total := count(ctx); total != 3 {
		t.Fatalf("rows after rollback = %d, want 3; the batch escaped the transaction", total)
	}
}
