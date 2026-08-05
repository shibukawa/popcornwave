package pwgen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// TestConfigBindGenIsCurrent regenerates the framework's own configbind
// bindings and compares them with the files in the tree.
//
// Those files are committed rather than generated at build time, so that the
// framework stays go-gettable: an application depending on it runs no code
// generation step. The cost is that a struct tag edited without regenerating
// leaves the two out of step, which is what this test catches. Regenerate by
// running it with PWGEN_WRITE=1.
func TestConfigBindGenIsCurrent(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	runner := generator.New(options)
	// Every committed binding, so a struct tag edited in one of them cannot
	// leave the generated file behind without this failing.
	for _, dir := range []string{"../../pw", "../../plugin/auth", "../../database/dynamo", "../../database/firestore"} {
		if os.Getenv("PWGEN_WRITE") != "" {
			if _, err := runner.GenerateConfigBind(dir, dir, ""); err != nil {
				t.Fatalf("%s: %v", dir, err)
			}
			continue
		}
		fresh, err := runner.GenerateConfigBind(dir, t.TempDir(), "")
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		want, err := os.ReadFile(fresh)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "configbind_gen.go"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s/configbind_gen.go is stale; rerun with PWGEN_WRITE=1", dir)
		}
	}
}
