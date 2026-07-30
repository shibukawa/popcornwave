package pwcli

import (
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// TestEngineSQLDialects pins the engine-to-dialect mapping. The two naming
// schemes agree everywhere except PostgreSQL, which tinybind spells
// postgresql, so a mapping that drifted would generate the wrong placeholders
// rather than fail to compile.
func TestEngineSQLDialects(t *testing.T) {
	want := map[string]string{
		engineSQLite:   sqlbind.DialectSQLite,
		enginePostgres: sqlbind.DialectPostgreSQL,
		engineMySQL:    sqlbind.DialectMySQL,
	}
	for _, name := range engineOrder {
		dialect := engineFor(name).SQLDialect
		if dialect != want[name] {
			t.Errorf("%s dialect = %q, want %q", name, dialect, want[name])
		}
		// tinybind rejects a dialect it does not serve, and the scaffold must
		// never write one it would reject.
		if err := sqlbind.ValidateDialect(dialect); err != nil {
			t.Errorf("%s dialect: %v", name, err)
		}
	}
	if len(engineOrder) != len(databaseEngines) {
		t.Fatalf("engineOrder covers %d of %d engines", len(engineOrder), len(databaseEngines))
	}
}
