package pw

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
)

func buildRuntimeHandler(handler http.Handler, server ServerConfig, security SecurityConfig, middleware MiddlewareConfig, resources pwruntime.Resources, publicFS ...fs.FS) (http.Handler, error) {
	trusted, err := compileTrustedProxies(server.TrustedProxies)
	if err != nil {
		return nil, err
	}
	// Extensions see the same resources the request handler will, so a
	// disabled or misconfigured capability fails during startup rather than on
	// the first request.
	extended, err := applyExtensions(pwruntime.WithResources(context.Background(), resources), handler)
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
	return middlewares.Track(result), nil
}

func writePanicProblem(w http.ResponseWriter, r *http.Request, err error) {
	if responseCommitted(w) {
		Logger(r.Context()).ErrorContext(r.Context(), "panic after response commit", "error", err)
		return
	}
	WriteProblem(w, r, InternalServerError(err))
}

func operationalEndpoints(next http.Handler, config ServerConfig, resources pwruntime.Resources) http.Handler {
	apiDoc := apiDocUI(config.APIDoc, config.OpenAPI)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case serveFrameworkScript(w, r):
			return
		case config.Health != "" && r.URL.Path == config.Health:
			writeOperationalStatus(w, r, true)
			return
		case config.Readiness != "" && r.URL.Path == config.Readiness:
			ready := true
			if resources.DB != nil {
				ctx, cancel := context.WithTimeout(r.Context(), time.Second)
				ready = resources.DB.PingContext(ctx) == nil
				cancel()
			}
			writeOperationalStatus(w, r, ready)
			return
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
