package testutil

import (
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pw"
)

// A server database is reachable by DSN, so WithMigrations applies its
// migrations directly instead of replaying the SQLite snapshot, whose script is
// SQLite DDL no other engine would accept. The test is opt-in because it needs
// a live server; see database/integration_test.go for the containers.
//
// The DSN must name a database dedicated to this suite. goose records applied
// versions by number, so a database carrying another project's version 1 makes
// this migration look applied and the schema never arrives.
func TestRunMigratesAServerDatabase(t *testing.T) {
	for _, engine := range []struct{ name, env string }{
		{name: "postgres", env: "PW_POSTGRES_TEST_DSN"},
		{name: "mysql", env: "PW_MYSQL_TEST_DSN"},
	} {
		t.Run(engine.name, func(t *testing.T) {
			dsn := os.Getenv(engine.env)
			if dsn == "" {
				t.Skipf("set %s to run this engine", engine.env)
			}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var count int
				db, ok := pw.DB(r.Context())
				if !ok {
					http.Error(w, "no pool", http.StatusInternalServerError)
					return
				}
				if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM notes").Scan(&count); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
			server := TestRun(t, handler, func(config *Config) {
				Update[pw.ServerConfig](config, func(value *pw.ServerConfig) {
					value.Public.Enabled = false
				})
				Update[pw.MiddlewareConfig](config, func(value *pw.MiddlewareConfig) {
					value.RDB = pw.RDBConfig{
						Enabled:        true,
						DSN:            dsn,
						ConnectTimeout: 10 * time.Second,
						MaxOpenConns:   4,
						MaxIdleConns:   2,
					}
				})
			}, WithMigrations("testdata/servermigrations"))

			// The handler reads the migrated table, so a 204 proves the schema
			// reached the server rather than a scratch SQLite file.
			response, err := server.Client().Get(server.URL + "/")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusNoContent {
				body, _ := io.ReadAll(response.Body)
				t.Fatalf("status = %d: %s", response.StatusCode, body)
			}

			// Applying twice must be a no-op, which is what lets several
			// TestRun calls share one prepared server database.
			second := TestRun(t, handler, func(config *Config) {
				Update[pw.ServerConfig](config, func(value *pw.ServerConfig) {
					value.Public.Enabled = false
				})
				Update[pw.MiddlewareConfig](config, func(value *pw.MiddlewareConfig) {
					value.RDB = pw.RDBConfig{
						Enabled:        true,
						DSN:            dsn,
						ConnectTimeout: 10 * time.Second,
						MaxOpenConns:   4,
						MaxIdleConns:   2,
					}
				})
			}, WithMigrations("testdata/servermigrations"))
			if second == nil {
				t.Fatal("second TestRun did not start")
			}
		})
	}
}
