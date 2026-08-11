//go:build pwdev

package pw

import (
	"github.com/shibukawa/popcornwave/pwdatabase"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// developmentTestFixture builds the wrapped handler against a real SQLite pool
// and a seed directory holding one dataset, plus a decoy file outside it.
func developmentTestFixture(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	seedDir := filepath.Join(root, "testdata", "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dataset := "member:\n- { id: 1, name: Frank }\n- { id: 2, name: Grace }\n"
	if err := os.WriteFile(filepath.Join(seedDir, "initial.yaml"), []byte(dataset), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "testdata", "outside.yaml"), []byte(dataset), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	connection, err := pwdatabase.OpenOne(RDBConnectionConfig{DSN: "sqlite://:memory:", ConnectTimeout: time.Second, MaxOpenConns: 1, MaxIdleConns: 1}, "test")
	if err != nil {
		t.Fatal(err)
	}
	db := connection.DB
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec("CREATE TABLE member (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}

	middleware := MiddlewareConfig{RDB: RDBConfig{
		Enabled:     true,
		Connections: []RDBConnectionConfig{{DSN: "sqlite://:memory:"}},
	}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	return developmentTestEndpoints(next, middleware, pwruntime.Resources{DB: db, DBDriver: "sqlite"})
}

func loopbackTestCall(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.RemoteAddr = "127.0.0.1:54321"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestDevelopmentTestEndpointsSeedAndAssert(t *testing.T) {
	handler := developmentTestFixture(t)

	if response := loopbackTestCall(handler, http.MethodPost, "/_pw/test/seed/initial"); response.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d, body %q", response.Code, response.Body.String())
	}
	if response := loopbackTestCall(handler, http.MethodGet, "/_pw/test/assert/initial"); response.Code != http.StatusNoContent {
		t.Fatalf("assert status = %d, body %q", response.Code, response.Body.String())
	}
}

func TestDevelopmentTestEndpointsRefuseEscapesAndStrangers(t *testing.T) {
	handler := developmentTestFixture(t)

	// The mux cleans dotted paths before matching, so a traversal URL never
	// reaches the handler. The containment check in testDataset answers any
	// caller that arrives around that cleaning; both layers are held to it.
	if response := loopbackTestCall(handler, http.MethodPost, "/_pw/test/seed/../outside"); response.Code == http.StatusNoContent {
		t.Fatal("a traversal URL was seeded")
	}
	escaping := httptest.NewRequest(http.MethodPost, "/_pw/test/seed/escape", nil)
	escaping.SetPathValue("dataset", "../outside")
	refused := httptest.NewRecorder()
	if _, ok := testDataset(refused, escaping); ok || refused.Code != http.StatusBadRequest {
		t.Fatalf("escaping dataset name: ok=%v status=%d, want a refused 400", ok, refused.Code)
	}
	if response := loopbackTestCall(handler, http.MethodPost, "/_pw/test/seed/missing"); response.Code != http.StatusNotFound {
		t.Fatalf("missing dataset status = %d, want 404", response.Code)
	}

	remote := httptest.NewRequest(http.MethodPost, "/_pw/test/seed/initial", nil)
	remote.RemoteAddr = "192.0.2.10:4444"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, remote)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("remote caller status = %d, want 403", recorder.Code)
	}

	relayed := httptest.NewRequest(http.MethodPost, "/_pw/test/seed/initial", nil)
	relayed.RemoteAddr = "127.0.0.1:4444"
	relayed.Header.Set("X-Forwarded-For", "198.51.100.7")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, relayed)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("relayed caller status = %d, want 403", recorder.Code)
	}
}

func TestDevelopmentTestEndpointsLeaveTheRestOfTheChainAlone(t *testing.T) {
	handler := developmentTestFixture(t)

	if response := loopbackTestCall(handler, http.MethodGet, "/members"); response.Code != http.StatusTeapot {
		t.Fatalf("application path status = %d, want the marker 418", response.Code)
	}
}

func TestDevelopmentTestEndpointsAbsentWithoutADatabase(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := developmentTestEndpoints(next, MiddlewareConfig{}, pwruntime.Resources{})
	if response := loopbackTestCall(handler, http.MethodPost, "/_pw/test/seed/initial"); response.Code != http.StatusTeapot {
		t.Fatalf("disabled RDB status = %d, want passthrough 418", response.Code)
	}
}

// The mismatch path needs real drift, which needs a second dataset the table
// does not match. It lives in its own test so the fixture stays one file.
func TestDevelopmentTestEndpointsAssertMismatchIs409(t *testing.T) {
	handler := developmentTestFixture(t)
	if response := loopbackTestCall(handler, http.MethodPost, "/_pw/test/seed/initial"); response.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d", response.Code)
	}
	expectation := "member:\n- { id: 1, name: Frank }\n"
	if err := os.WriteFile(filepath.Join("testdata", "seed", "only_frank.yaml"), []byte(expectation), 0o644); err != nil {
		t.Fatal(err)
	}
	response := loopbackTestCall(handler, http.MethodGet, "/_pw/test/assert/only_frank")
	if response.Code != http.StatusConflict {
		t.Fatalf("mismatch status = %d, want 409", response.Code)
	}
	if body := response.Body.String(); !strings.Contains(body, "member") {
		t.Fatalf("diff does not name the table:\n%s", body)
	}
}
