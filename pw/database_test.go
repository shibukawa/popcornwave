package pw

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwconfig"
)

func TestDatabaseTarget(t *testing.T) {
	target, err := pwconfig.Target("sqlite://app.db")
	if err != nil {
		t.Fatal(err)
	}
	if target.Dialect != "sqlite" || target.DataSource != "app.db" {
		t.Fatalf("target = %q, %q", target.Dialect, target.DataSource)
	}
	if _, err := pwconfig.Target("app.db"); err == nil {
		t.Fatal("DSN without scheme was accepted")
	}
}

// TestDatabaseTargetUnlinkedEngine asserts that an engine the framework ships
// but this binary did not link reports the import to add, rather than looking
// like an unknown database. This test binary links SQLite explicitly in
// database_engine_test.go, so PostgreSQL represents an omitted engine here.
func TestDatabaseTargetUnlinkedEngine(t *testing.T) {
	_, err := pwconfig.Target("postgres://user:secret@127.0.0.1:5432/app")
	if err == nil {
		t.Fatal("an unlinked engine was accepted")
	}
	if !strings.Contains(err.Error(), "database/postgres") {
		t.Fatalf("error does not name the import to add: %v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked the DSN password: %v", err)
	}
}

func TestConfiguredDatabaseDSN(t *testing.T) {
	replaceMiddlewareConfig(t, RDBConfig{
		Enabled:     true,
		Connections: []RDBConnectionConfig{{DSN: "sqlite://app.db"}},
	})

	dsn, err := configuredDatabaseDSN()
	if err != nil {
		t.Fatal(err)
	}
	if dsn != "sqlite://app.db" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestConfiguredDatabaseDSNRequiresEnabledRDB(t *testing.T) {
	replaceMiddlewareConfig(t, RDBConfig{Enabled: false})

	if _, err := configuredDatabaseDSN(); err == nil {
		t.Fatal("disabled RDB reported a DSN")
	}
}

func TestConfiguredDatabaseDSNRejectsMalformedDSN(t *testing.T) {
	replaceMiddlewareConfig(t, RDBConfig{
		Enabled:     true,
		Connections: []RDBConnectionConfig{{DSN: "app.db"}},
	})

	if _, err := configuredDatabaseDSN(); err == nil {
		t.Fatal("malformed DSN was accepted")
	}
}

// replaceMiddlewareConfig installs a middleware configuration value directly so
// the DSN accessor can be tested without parsing a project configuration.
func replaceMiddlewareConfig(t *testing.T, rdb RDBConfig) {
	t.Helper()
	swapConfigForTest(t, MiddlewareConfig{RDB: rdb})
}
