package pw

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

type responseTracker struct {
	http.ResponseWriter
	status int
	bytes  int64
	wrote  bool
}

func (w *responseTracker) WriteHeader(status int) {
	if w.wrote {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status, w.wrote = status, true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseTracker) Write(body []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(body)
	w.bytes += int64(count)
	return count, err
}

func (w *responseTracker) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseTracker) Committed() bool             { return w.wrote }
func (w *responseTracker) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
func (w *responseTracker) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
func (w *responseTracker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("popcornwave: response writer does not support hijacking")
	}
	return hijacker.Hijack()
}
func (w *responseTracker) Push(target string, options *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}
func (w *responseTracker) ReadFrom(reader io.Reader) (int64, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		count, err := readerFrom.ReadFrom(reader)
		w.bytes += count
		return count, err
	}
	count, err := io.Copy(struct{ io.Writer }{w}, reader)
	return count, err
}

func trackResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&responseTracker{ResponseWriter: w}, r)
	})
}

func buildRuntimeHandler(handler http.Handler, server ServerConfig, security SecurityConfig, middleware MiddlewareConfig, resources pwruntime.Resources, publicFS ...fs.FS) (http.Handler, error) {
	trusted, err := compileTrustedProxies(server.TrustedProxies)
	if err != nil {
		return nil, err
	}
	result := operationalEndpoints(handler, server, resources)
	if server.Public.Enabled {
		var embedded fs.FS
		if len(publicFS) > 0 {
			embedded = publicFS[0]
		}
		result = publicAssetHandler(result, server.Public, embedded)
	}
	if server.MaxRequestBody > 0 {
		result = requestBodyLimit(result, server.MaxRequestBody)
	}
	if middleware.RequestTimeout > 0 {
		result = requestTimeout(result, middleware.RequestTimeout)
	}
	if security.Headers.Enabled {
		result = securityHeaders(result, security.Headers, trusted)
	}
	if middleware.Recovery {
		result = recoveryMiddleware(result)
	}
	if middleware.AccessLog {
		result = accessLogMiddleware(result)
	}
	if middleware.RequestID {
		result = requestIDMiddleware(result)
	}
	result = injectResources(resources)(result)
	return trackResponse(result), nil
}

func requestBodyLimit(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

func requestTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		status, bytes := http.StatusOK, int64(0)
		if tracker, ok := w.(*responseTracker); ok {
			status, bytes = tracker.Status(), tracker.bytes
		}
		Logger(r.Context()).InfoContext(r.Context(), "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", bytes,
			"duration", time.Since(start),
		)
	})
}

func securityHeaders(next http.Handler, config SecurityHeadersConfig, trusted []*net.IPNet) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.ContentTypeOptions {
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		switch strings.ToLower(config.FrameOptions) {
		case "deny":
			w.Header().Set("X-Frame-Options", "DENY")
		case "sameorigin":
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		}
		w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
		setOptionalHeader(w.Header(), "Content-Security-Policy", config.ContentSecurityPolicy)
		setOptionalHeader(w.Header(), "Content-Security-Policy-Report-Only", config.ContentSecurityPolicyReportOnly)
		setOptionalHeader(w.Header(), "Permissions-Policy", config.PermissionsPolicy)
		if config.HSTS.Enabled && requestIsHTTPS(r, trusted) {
			value := "max-age=" + strconv.FormatInt(int64(config.HSTS.MaxAge/time.Second), 10)
			if config.HSTS.IncludeSubdomains {
				value += "; includeSubDomains"
			}
			if config.HSTS.Preload {
				value += "; preload"
			}
			w.Header().Set("Strict-Transport-Security", value)
		}
		next.ServeHTTP(w, r)
	})
}

func setOptionalHeader(header http.Header, name, value string) {
	if value != "" {
		header.Set(name, value)
	}
}

func requestIsHTTPS(r *http.Request, trusted []*net.IPNet) bool {
	if r.TLS != nil {
		return true
	}
	if !trustedRemote(r.RemoteAddr, trusted) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func trustedRemote(remote string, trusted []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func operationalEndpoints(next http.Handler, config ServerConfig, resources pwruntime.Resources) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case config.Health.Enabled && r.URL.Path == config.Health.Path:
			writeOperationalStatus(w, r, true)
			return
		case config.Readiness.Enabled && r.URL.Path == config.Readiness.Path:
			ready := true
			if resources.DB != nil {
				ctx, cancel := context.WithTimeout(r.Context(), time.Second)
				ready = resources.DB.PingContext(ctx) == nil
				cancel()
			}
			writeOperationalStatus(w, r, ready)
			return
		case config.OpenAPI.Enabled && r.URL.Path == config.OpenAPI.Path:
			if !operationalMethod(w, r) {
				return
			}
			if r.Method == http.MethodHead {
				OpenAPIJSON(headResponseWriter{ResponseWriter: w}, r)
			} else {
				OpenAPIJSON(w, r)
			}
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
