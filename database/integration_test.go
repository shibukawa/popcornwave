package database_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/database"
)

// The server engines need a live server, so their tests are opt-in:
//
//	docker run -d --name pgtest -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=pw \
//	    -e POSTGRES_DB=pw -p 55432:5432 postgres:17
//	docker run -d --name mytest -e MYSQL_ROOT_PASSWORD=root -e MYSQL_USER=pw \
//	    -e MYSQL_PASSWORD=pw -e MYSQL_DATABASE=pw -p 53306:3306 mysql:8
//
//	PW_POSTGRES_TEST_DSN='postgres://pw:pw@127.0.0.1:55432/pw?sslmode=disable' \
//	PW_MYSQL_TEST_DSN='mysql://pw:pw@tcp(127.0.0.1:53306)/pw' \
//	    go test ./database/
const (
	postgresDSNEnv = "PW_POSTGRES_TEST_DSN"
	mysqlDSNEnv    = "PW_MYSQL_TEST_DSN"
)

// TestEngineContract runs one suite against every engine, which is what makes
// the three interchangeable rather than three separately plausible ones. SQLite
// needs no server, so it always runs and the rest join when configured.
func TestEngineContract(t *testing.T) {
	for _, engine := range []struct {
		dialect string
		dsn     string
	}{
		{dialect: "sqlite", dsn: "sqlite://" + t.TempDir() + "/contract.db"},
		{dialect: "postgres", dsn: os.Getenv(postgresDSNEnv)},
		{dialect: "mysql", dsn: os.Getenv(mysqlDSNEnv)},
	} {
		t.Run(engine.dialect, func(t *testing.T) {
			if engine.dsn == "" {
				t.Skipf("set %s_TEST_DSN to run this engine", engine.dialect)
			}
			target, err := database.Resolve(engine.dsn)
			if err != nil {
				t.Fatal(err)
			}
			if target.Dialect != engine.dialect {
				t.Fatalf("dialect = %q, want %q", target.Dialect, engine.dialect)
			}
			db, err := target.Open()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				t.Fatal(err)
			}
			runEngineContract(ctx, t, db, engine.dialect)
		})
	}
}

func runEngineContract(ctx context.Context, t *testing.T, db *sql.DB, dialect string) {
	t.Helper()
	table := "pw_contract_" + dialect
	exec(ctx, t, db, "DROP TABLE IF EXISTS "+table)
	t.Cleanup(func() { exec(context.Background(), t, db, "DROP TABLE IF EXISTS "+table) })
	exec(ctx, t, db, "CREATE TABLE "+table+" (id INTEGER PRIMARY KEY, name "+textColumn(dialect)+" NOT NULL)")

	// Placeholders differ by dialect, which is exactly why generated SQL is not
	// portable between them; the contract here is the driver, not the syntax.
	insert := "INSERT INTO " + table + " (id, name) VALUES (" + placeholder(dialect, 1) + ", " + placeholder(dialect, 2) + ")"
	exec(ctx, t, db, insert, 1, "first")

	var name string
	query := "SELECT name FROM " + table + " WHERE id = " + placeholder(dialect, 1)
	if err := db.QueryRowContext(ctx, query, 1).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "first" {
		t.Fatalf("name = %q", name)
	}
	if err := db.QueryRowContext(ctx, query, 404).Scan(&name); err != sql.ErrNoRows {
		t.Fatalf("missing row error = %v, want sql.ErrNoRows", err)
	}

	// A rolled-back savepoint must leave the outer transaction usable, which is
	// what requirement:parallel-database-tests runs every test inside.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, insert, 2, "outer"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT pw_contract"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, insert, 3, "inner"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT pw_contract"); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 2 {
		t.Fatalf("rows after savepoint rollback = %d, want 2", remaining)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func exec(ctx context.Context, t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, statement, args...); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

func textColumn(dialect string) string {
	if dialect == "mysql" {
		// MySQL cannot index a TEXT column without a prefix length, and
		// VARCHAR is what the scaffolded schema uses for the same reason.
		return "VARCHAR(255)"
	}
	return "TEXT"
}

func placeholder(dialect string, position int) string {
	if dialect == "postgres" {
		return "$" + string(rune('0'+position))
	}
	return "?"
}

// TestNativeContract runs the native-path contract against PostgreSQL, the one
// engine that registers a native opener. It exercises what the runtime needs
// from a NativeDB: ping, exec with an affected count, the sqlbind cursor, a
// committed and a rolled-back transaction, and the savepoint statements the
// transaction scope nests with.
func TestNativeContract(t *testing.T) {
	dsn := os.Getenv(postgresDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to run this engine", postgresDSNEnv)
	}
	target, err := database.Resolve(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !target.Native() {
		t.Fatal("the postgres engine registered no native opener")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	native, err := target.OpenNative(ctx, database.PoolBounds{MaxOpenConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Close() })
	if err := native.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	table := "pw_native_contract"
	mustExec := func(executor interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}, statement string, args ...any) sql.Result {
		t.Helper()
		result, err := executor.ExecContext(ctx, statement, args...)
		if err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
		return result
	}
	mustExec(native, "DROP TABLE IF EXISTS "+table)
	t.Cleanup(func() {
		_, _ = native.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+table)
	})
	mustExec(native, "CREATE TABLE "+table+" (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")

	result := mustExec(native, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 1, "first")
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		t.Fatalf("affected = %d, %v; want 1", affected, err)
	}
	if _, err := result.LastInsertId(); err == nil {
		t.Fatal("LastInsertId succeeded; PostgreSQL cannot report it")
	}

	rows, err := native.QueryRows(ctx, "SELECT id, name FROM "+table+" ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0] != "id" || columns[1] != "name" {
		t.Fatalf("columns = %v", columns)
	}
	if !rows.Next() {
		t.Fatalf("no rows: %v", rows.Err())
	}
	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatal(err)
	}
	if id != 1 || name != "first" {
		t.Fatalf("row = %d, %q", id, name)
	}
	if rows.Next() {
		t.Fatal("unexpected second row")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	// The QueryContext half of the executor contract is the explanatory stub.
	if _, err := native.QueryContext(ctx, "SELECT 1"); err == nil {
		t.Fatal("QueryContext succeeded on a native executor")
	}

	// A savepoint rolled back inside a native transaction leaves the outer
	// transaction usable, which is what the transaction scope nests with.
	tx, err := native.BeginTx(ctx, database.NativeTxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(tx, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 2, "outer")
	mustExec(tx, "SAVEPOINT pw_native")
	mustExec(tx, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 3, "inner")
	mustExec(tx, "ROLLBACK TO SAVEPOINT pw_native")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback after commit = %v, want nil", err)
	}
	count := func() int {
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
	if total := count(); total != 2 {
		t.Fatalf("rows after savepoint rollback = %d, want 2", total)
	}

	// A rolled-back transaction leaves nothing behind.
	tx, err = native.BeginTx(ctx, database.NativeTxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(tx, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", 4, "discarded")
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if total := count(); total != 2 {
		t.Fatalf("rows after rollback = %d, want 2", total)
	}

	// A read-only transaction refuses a write, which is what a replica
	// connection begins at depth 0.
	tx, err = native.BeginTx(ctx, database.NativeTxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+" (id, name) VALUES (5, 'refused')"); err == nil {
		t.Fatal("a read-only native transaction accepted a write")
	}
	_ = tx.Rollback(ctx)
}

// BenchmarkPostgresQuery compares the two paths the postgres engine registers
// on one SELECT, which is the seam the native opener exists to relieve: the
// database/sql pool mutex, the per-conn mutex, and driver.Value boxing.
func BenchmarkPostgresQuery(b *testing.B) {
	dsn := os.Getenv(postgresDSNEnv)
	if dsn == "" {
		b.Skipf("set %s to run this benchmark", postgresDSNEnv)
	}
	target, err := database.Resolve(dsn)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.Run("stdlib", func(b *testing.B) {
		db, err := target.Open()
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = db.Close() }()
		db.SetMaxOpenConns(8)
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				var n int
				if err := db.QueryRowContext(ctx, "SELECT $1::int", 1).Scan(&n); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	b.Run("native", func(b *testing.B) {
		native, err := target.OpenNative(ctx, database.PoolBounds{MaxOpenConns: 8})
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = native.Close() }()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				rows, err := native.QueryRows(ctx, "SELECT $1::int", 1)
				if err != nil {
					b.Fatal(err)
				}
				var n int
				if !rows.Next() {
					b.Fatal(rows.Err())
				}
				if err := rows.Scan(&n); err != nil {
					b.Fatal(err)
				}
				_ = rows.Close()
			}
		})
	})
}
