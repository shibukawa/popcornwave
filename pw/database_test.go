package pw

import (
	"reflect"
	"testing"

	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

func TestDatabaseTarget(t *testing.T) {
	driver, dsn, err := databaseTarget("sqlite://app.db")
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "app.db" {
		t.Fatalf("target = %q, %q", driver, dsn)
	}
	if _, _, err := databaseTarget("app.db"); err == nil {
		t.Fatal("DSN without scheme was accepted")
	}
}

func TestConfiguredDatabaseDSN(t *testing.T) {
	replaceMiddlewareConfig(t, RDBConfig{Enabled: true, DSN: "sqlite://app.db"})

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
	replaceMiddlewareConfig(t, RDBConfig{Enabled: true, DSN: "app.db"})

	if _, err := configuredDatabaseDSN(); err == nil {
		t.Fatal("malformed DSN was accepted")
	}
}

// replaceMiddlewareConfig installs a middleware configuration value directly so
// the DSN accessor can be tested without parsing a project configuration.
func replaceMiddlewareConfig(t *testing.T, rdb RDBConfig) {
	t.Helper()
	key := reflect.TypeFor[MiddlewareConfig]()
	value := MiddlewareConfig{RDB: rdb}
	configState.Lock()
	previous, existed := configState.entries[key]
	configState.entries[key] = configEntry{prefix: "middleware", ptr: &value}
	configState.Unlock()
	t.Cleanup(func() {
		configState.Lock()
		if existed {
			configState.entries[key] = previous
		} else {
			delete(configState.entries, key)
		}
		configState.Unlock()
	})
}
