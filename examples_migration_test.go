package petitweb_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/plugin/session/rdb"
)

// TestExampleFrameworkMigrationsMatchOwners keeps the migration files carried by
// the examples identical to the SQL their owning packages publish. The files are
// written by hand today and scaffolded by api:cli-init once the authentication
// modes settle; either way they must not drift from the tables the runtime
// verifies at startup.
func TestExampleFrameworkMigrationsMatchOwners(t *testing.T) {
	owned := map[string]string{
		rdb.MigrationFileName:  rdb.MigrationSQL(""),
		auth.MigrationFileName: auth.MigrationSQL(),
	}
	for _, example := range []string{"oidclogin"} {
		directory := filepath.Join("examples", example, "migrations")
		for name, want := range owned {
			path := filepath.Join(directory, name)
			got, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			if string(got) != want {
				t.Errorf("%s does not match the SQL published by its owning package", path)
			}
		}
	}
}
