//go:build pwdev

package pw

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/shibukawa/popcornwave/internal/dbseed"
	"github.com/shibukawa/popcornwave/pwconfig"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// developmentTestPrefix is where the seed and assert endpoints live, inside the
// namespace serveReservedPath closes. The wrapper sits above the runtime chain,
// so a claimed path never reaches the closure, and an unclaimed one is still
// answered 404 there rather than leaking into application routing.
const developmentTestPrefix = "/_pw/test/"

// developmentTestEndpoints mounts the dataset seed and assert API on the
// application's own listener.
//
// A browser suite drives the application from another process, which is the
// one caller the existing seeding routes serve badly: pw seed pays an
// application compile per call just to learn the DSN, and an in-memory
// database it cannot reach at all. Serving the same dataset files from inside
// the process makes the running pool itself the target, so what the suite
// seeds is exactly what the handlers read.
//
// Three locks, matching policy:devidp-safety: the pwdev build mode (this
// file), a declared development environment, and a loopback caller carrying no
// forwarding header. Anything short of all three leaves the handler chain
// untouched, and the reserved namespace answers 404 as if the endpoints had
// never been built.
func developmentTestEndpoints(next http.Handler, middleware MiddlewareConfig, resources pwruntime.Resources) http.Handler {
	if !Development() || !middleware.RDB.Enabled {
		return next
	}
	dsn, err := middleware.RDB.MigrationDSN()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pw: test data endpoints:", err)
		return next
	}
	dialect, err := dbseed.ResolveDialect(dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pw: test data endpoints:", err)
		return next
	}
	pool := migrationExecutor(middleware.RDB, resources)
	if pool == nil {
		return next
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+developmentTestPrefix+"seed/{dataset...}", func(w http.ResponseWriter, r *http.Request) {
		paths, ok := testDataset(w, r)
		if !ok {
			return
		}
		if err := dbseed.Apply(r.Context(), pool, dialect, false, paths); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET "+developmentTestPrefix+"assert/{dataset...}", func(w http.ResponseWriter, r *http.Request) {
		paths, ok := testDataset(w, r)
		if !ok {
			return
		}
		matched, report, err := dbseed.Assert(r.Context(), pool, dialect, false, paths)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !matched {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, report)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, developmentTestPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		if !loopbackTestRequest(r) {
			http.Error(w, "the test data endpoints answer loopback requests only", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// testDataset resolves the dataset named by the request path and keeps the
// answer inside the seed directory. pw seed trusts its arguments because an
// operator typed them; an HTTP caller is not that operator, so a name that
// resolves outside testdata/seed is refused rather than read.
func testDataset(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	name := r.PathValue("dataset")
	paths, err := dbseed.Resolve(dbseed.DefaultDir, []string{name})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return nil, false
	}
	for _, path := range paths {
		rel, err := filepath.Rel(dbseed.DefaultDir, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			http.Error(w, "dataset name escapes the seed directory", http.StatusBadRequest)
			return nil, false
		}
	}
	return paths, true
}

// migrationExecutor picks the pool seed data lands in: the migration group,
// where the schema is — the same routing pw seed and the test helpers use.
// The pool is handed over as a dbseed executor, so a native connection serves
// these endpoints the same way a *sql.DB does. Resolution failures fall back
// to the default pool rather than erroring, because the configuration that
// could fail here has already survived startup validation.
func migrationExecutor(config RDBConfig, resources pwruntime.Resources) dbseed.Executor {
	fallback := func() dbseed.Executor {
		if resources.DB != nil {
			return dbseed.FromSQL(resources.DB)
		}
		for _, connection := range resources.Connections.Connections() {
			if !connection.ReadOnly {
				return dbseed.FromRuntime(connection.Executor())
			}
		}
		return nil
	}
	if resources.Connections == nil {
		return fallback()
	}
	connections, err := pwconfig.ResolveConnections(config)
	if err != nil {
		return fallback()
	}
	group, err := pwconfig.ResolveMigrationGroup(config, connections)
	if err != nil {
		return fallback()
	}
	for _, connection := range resources.Connections.Connections() {
		if connection.Group == group && !connection.ReadOnly {
			return dbseed.FromRuntime(connection.Executor())
		}
	}
	return fallback()
}

// loopbackTestRequest admits only a caller on this machine. A forwarding
// header means RemoteAddr describes a relay rather than the caller, and a
// relay is exactly what these endpoints must never be reachable through, so
// there is no opt-out — matching the listen rule of policy:devidp-safety.
func loopbackTestRequest(r *http.Request) bool {
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-Ip"} {
		if r.Header.Get(name) != "" {
			return false
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
