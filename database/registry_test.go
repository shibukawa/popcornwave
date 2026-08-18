package database_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/database"
	_ "github.com/shibukawa/popcornweb/database/mysql"
	_ "github.com/shibukawa/popcornweb/database/postgres"
	_ "github.com/shibukawa/popcornweb/database/sqlite"
)

// TestResolve covers the two things a scheme decides: which dialect the rest of
// the framework sees, and how much of the DSN the engine receives.
func TestResolve(t *testing.T) {
	for _, testCase := range []struct {
		dsn        string
		dialect    string
		dataSource string
	}{
		{dsn: "sqlite://app.db", dialect: "sqlite", dataSource: "app.db"},
		{dsn: "sqlite://:memory:", dialect: "sqlite", dataSource: ":memory:"},
		// An alias resolves to the one dialect, so it cannot produce a second
		// savepoint, EXPLAIN, or migration behavior.
		{dsn: "sqlite3://app.db", dialect: "sqlite", dataSource: "app.db"},
		{
			dsn:        "postgres://user:pass@host:5432/app?sslmode=verify-full",
			dialect:    "postgres",
			dataSource: "postgres://user:pass@host:5432/app?sslmode=verify-full",
		},
		{
			dsn:        "postgresql://user:pass@host:5432/app",
			dialect:    "postgres",
			dataSource: "postgresql://user:pass@host:5432/app",
		},
		// A go-sql-driver DSN is not a URL, so the scheme has to come off.
		{
			dsn:        "mysql://user:pass@tcp(host:3306)/app",
			dialect:    "mysql",
			dataSource: "user:pass@tcp(host:3306)/app",
		},
		{dsn: "  sqlite://app.db  ", dialect: "sqlite", dataSource: "app.db"},
	} {
		target, err := database.Resolve(testCase.dsn)
		if err != nil {
			t.Fatalf("resolve %q: %v", testCase.dsn, err)
		}
		if target.Dialect != testCase.dialect || target.DataSource != testCase.dataSource {
			t.Fatalf("resolve %q = %q, %q; want %q, %q",
				testCase.dsn, target.Dialect, target.DataSource, testCase.dialect, testCase.dataSource)
		}
	}
}

func TestResolveRejectsMalformedDSN(t *testing.T) {
	for _, dsn := range []string{"", "app.db", "://app.db", "sqlite://"} {
		if _, err := database.Resolve(dsn); err == nil {
			t.Fatalf("resolve accepted %q", dsn)
		}
	}
}

// TestResolveUnknownScheme asserts the error names what this binary can open,
// so an operator sees the choice rather than a driver-not-found from the depths
// of database/sql.
func TestResolveUnknownScheme(t *testing.T) {
	_, err := database.Resolve("cassandra://user:secret@host/app")
	if err == nil {
		t.Fatal("resolve accepted an unknown scheme")
	}
	message := err.Error()
	for _, want := range []string{"cassandra", "sqlite", "postgres", "mysql"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error does not mention %q: %v", want, err)
		}
	}
	if strings.Contains(message, "secret") {
		t.Fatalf("error leaked the DSN password: %v", err)
	}
}

func TestSchemes(t *testing.T) {
	got := strings.Join(database.Schemes(), ",")
	want := "mysql,postgres,postgresql,sqlite,sqlite3"
	if got != want {
		t.Fatalf("schemes = %q; want %q", got, want)
	}
}

// TestOpenUnresolvedTarget guards the zero value, which a caller can reach by
// ignoring a Resolve error.
func TestOpenUnresolvedTarget(t *testing.T) {
	if _, err := (database.Target{}).Open(); err == nil {
		t.Fatal("an unresolved target opened a pool")
	}
}
