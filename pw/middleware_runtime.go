package pw

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
)

func buildRuntimeHandler(handler http.Handler, server ServerConfig, security SecurityConfig, middleware MiddlewareConfig, resources pwruntime.Resources, tracing bool, publicFS ...fs.FS) (http.Handler, error) {
	trusted, err := compileTrustedProxies(server.TrustedProxies)
	if err != nil {
		return nil, err
	}
	// Extensions see the same resources the request handler will, so a
	// disabled or misconfigured capability fails during startup rather than on
	// the first request.
	//
	// The documentation endpoints go underneath them, so the session and the
	// authentication guard reach the OpenAPI document exactly as they reach an
	// application route. The probes stay above, where nothing authenticates.
	extended, err := applyExtensions(pwruntime.WithResources(context.Background(), resources), documentationEndpoints(handler, server))
	if err != nil {
		return nil, err
	}
	result := operationalEndpoints(extended, server, resources)
	if server.Public.Enabled {
		var embedded fs.FS
		if len(publicFS) > 0 {
			embedded = publicFS[0]
		}
		assets, err := middlewares.PublicAssets(server.Public, embedded)
		if err != nil {
			return nil, err
		}
		result = assets(result)
	}
	if server.MaxRequestBody > 0 {
		result = middlewares.MaxRequestBody(server.MaxRequestBody)(result)
	}
	if middleware.RequestTimeout > 0 {
		result = middlewares.RequestTimeout(middleware.RequestTimeout)(result)
	}
	if security.Headers.Enabled {
		headers, err := middlewares.SecurityHeaders(security.Headers, middlewares.WithTrustedProxies(trusted))
		if err != nil {
			return nil, err
		}
		result = headers(result)
	}
	if middleware.Recovery {
		result = middlewares.Recover(writePanicProblem)(result)
	}
	if middleware.AccessLog {
		result = middlewares.AccessLog()(result)
	}
	if middleware.RequestID {
		result = middlewares.RequestID()(result)
	}
	result = middlewares.InjectResources(resources)(result)
	// Tracing wraps everything the framework installs, so the request root span
	// covers the whole chain and every record taken inside it correlates. It is
	// omitted when nothing exports, because an unsampled span is pure cost.
	if tracing {
		result = middlewares.Otel()(result)
	}
	return middlewares.Track(result), nil
}

func writePanicProblem(w http.ResponseWriter, r *http.Request, err error) {
	if responseCommitted(w) {
		Logger(r.Context()).Log(r.Context(), LevelError, "panic after response commit", String("error", err.Error()))
		return
	}
	WriteProblem(w, r, InternalServerError(err))
}

// operationalEndpoints answers the framework assets and the two probes, above
// every extension. Health and readiness reveal only status and are reachable by
// anything that can reach the port, which is what a liveness probe needs and
// what keeps a dependency outage from turning into a restart loop.
func operationalEndpoints(next http.Handler, config ServerConfig, resources pwruntime.Resources) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case serveFrameworkScript(w, r):
			return
		case config.Health != "" && r.URL.Path == config.Health:
			writeOperationalStatus(w, r, true)
			return
		case config.Readiness != "" && r.URL.Path == config.Readiness:
			writeOperationalStatus(w, r, databasesReady(r.Context(), resources))
			return
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// documentationEndpoints answers the generated OpenAPI document and the UI over
// it. Unlike the probes it is mounted beneath the extension chain, because an
// API description is a map of the whole application surface and belongs behind
// whatever protects the routes it describes. Reaching it therefore costs a
// session where the configuration says so, and a test that authenticates
// reaches it the same way its own routes are reached.
//
// A configuration that serves neither returns the handler unchanged, so the
// common case adds no frame to the chain.
func documentationEndpoints(next http.Handler, config ServerConfig) http.Handler {
	apiDoc := apiDocUI(config.APIDoc, config.OpenAPI)
	if config.OpenAPI == "" && apiDoc == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case config.OpenAPI != "" && r.URL.Path == config.OpenAPI:
			if !operationalMethod(w, r) {
				return
			}
			if r.Method == http.MethodHead {
				OpenAPIJSON(headResponseWriter{ResponseWriter: w}, r)
			} else {
				OpenAPIJSON(w, r)
			}
			return
		case apiDoc != nil && r.URL.Path == config.APIDocPath:
			if !operationalMethod(w, r) {
				return
			}
			apiDoc.ServeHTTP(w, r)
			return
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func writeOperationalStatus(w http.ResponseWriter, r *http.Request, healthy bool) {
	if !operationalMethod(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	status := http.StatusOK
	body := "ok\n"
	if !healthy {
		status, body = http.StatusServiceUnavailable, "unavailable\n"
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, body)
	}
}

func operationalMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return false
}

type headResponseWriter struct{ http.ResponseWriter }

func (headResponseWriter) Write(body []byte) (int, error) { return len(body), nil }

// databasesReady pings every configured connection. A replica that cannot
// answer makes the instance unready, because the default group is the one the
// application reads from.
func databasesReady(parent context.Context, resources pwruntime.Resources) bool {
	pools := []*sql.DB{}
	if connections := resources.Connections.Connections(); len(connections) > 0 {
		for _, connection := range connections {
			pools = append(pools, connection.DB)
		}
	} else if resources.DB != nil {
		pools = append(pools, resources.DB)
	}
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	for _, pool := range pools {
		if pool.PingContext(ctx) != nil {
			return false
		}
	}
	return true
}
