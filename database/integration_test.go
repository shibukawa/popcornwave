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
