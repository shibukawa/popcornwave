package testutil

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/tinybind-go/configbind"
)

type fixtureConfig struct {
	Name   string
	Labels []string
}

func init() {
	configbind.Register[fixtureConfig](configbind.Definition{
		TypeName:  "github.com/shibukawa/popcornwave/testutil.fixtureConfig",
		Prefix:    "fixture",
		KnownKeys: []string{"fixture.name"},
		Defaults:  map[string]string{"fixture.name": "global"},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			config := dst.(*fixtureConfig)
			config.Name, _ = overlay.GetString("fixture.name")
			config.Labels = []string{"original"}
			return nil
		},
	})
	pw.RegisterConfig[fixtureConfig]("fixture")
}

func TestRunCopiesAndCustomizesArbitraryConfig(t *testing.T) {
	schemaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(schemaDir, "001_counter.sql"), []byte(
		"CREATE TABLE counter (value INTEGER NOT NULL);",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "002_seed.sql"), []byte(
		"INSERT INTO counter VALUES (7);",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	var sawDefaultPort bool
	server := TestRun(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := pw.Config[fixtureConfig](r.Context())
		runtimeServer := pw.Config[pw.ServerConfig](r.Context())
		db, ok := pw.DB(r.Context())
		if !ok {
			t.Fatal("test database is missing")
		}
		var value int
		if err := db.QueryRowContext(r.Context(), "SELECT value FROM counter").Scan(&value); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(config.Name + ":" + strings.Join(config.Labels, ",") + ":" +
			strconv.Itoa(runtimeServer.Port) + ":" + strconv.Itoa(value)))
	}), func(config *Config) {
		sawDefaultPort = Get[pw.ServerConfig](config).Port == -1
		Update[fixtureConfig](config, func(value *fixtureConfig) {
			value.Name = "copied"
			value.Labels[0] = "isolated"
		})
		Update[pw.ServerConfig](config, func(value *pw.ServerConfig) {
			value.Public.Enabled = false
		})
		Update[pw.MiddlewareConfig](config, func(value *pw.MiddlewareConfig) {
			value.RDB = pw.RDBConfig{
				Enabled:         true,
				DSN:             "sqlite://:memory:",
				AutoTransaction: false,
				ConnectTimeout:  time.Second,
				MaxOpenConns:    1,
				MaxIdleConns:    1,
			}
		})
	}, WithSchemaDir(schemaDir))
	if !sawDefaultPort {
		t.Fatal("customizer did not observe default port -1")
	}
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	buffer, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(buffer)
	actualPort := Get[pw.ServerConfig](server.Config).Port
	want := "copied:isolated:" + strconv.Itoa(actualPort) + ":7"
	if body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
	global := pw.Config[fixtureConfig](nil)
	if global.Name != "global" || strings.Join(global.Labels, ",") != "original" {
		t.Fatalf("global config was mutated: %#v", global)
	}
}
