package petitweb_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/sessionstore"

	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
)

// TestExampleFrameworkMigrationsMatchOwners keeps the migration files carried by
// the examples identical to the SQL their owning packages publish, so a sample
// cannot drift from the tables the runtime verifies at startup.
//
// A file is located by its name rather than by its version, because
// rule:framework-owned-tables reserves no version range: the version is whatever
// was free in that project when api:cli-init or api:cli-add wrote the file.
func TestExampleFrameworkMigrationsMatchOwners(t *testing.T) {
	owned := map[string]string{
		sessionstore.MigrationName: mustSessionMigration(),
		auth.MigrationName:         mustAuthMigration(),
	}
	for _, example := range []string{"oidclogin", "passkeylogin"} {
		directory := filepath.Join("examples", example, "migrations")
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		found := make(map[string]bool, len(owned))
		for _, entry := range entries {
			name, ok := migrationName(entry.Name())
			if !ok {
				continue
			}
			want, owns := owned[name]
			if !owns {
				continue
			}
			found[name] = true
			path := filepath.Join(directory, entry.Name())
			got, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			if string(got) != want {
				t.Errorf("%s does not match the SQL published by its owning package", path)
			}
		}
		for name := range owned {
			if !found[name] {
				t.Errorf("%s carries no migration named %s", directory, name)
			}
		}
	}
}

// migrationName strips the version prefix and the extension of a migration file.
func migrationName(fileName string) (string, bool) {
	base, ok := strings.CutSuffix(fileName, ".sql")
	if !ok {
		return "", false
	}
	_, name, ok := strings.Cut(base, "_")
	return name, ok
}

// mustSessionMigration and mustAuthMigration are the SQLite migrations the
// scaffold writes, which is the dialect these fixtures use.
func mustSessionMigration() string {
	migration, err := sessionstore.MigrationSQL("sqlite", "popcornwave_session")
	if err != nil {
		panic(err)
	}
	return migration
}

func mustAuthMigration() string {
	migration, err := auth.MigrationSQL("sqlite")
	if err != nil {
		panic(err)
	}
	return migration
}
