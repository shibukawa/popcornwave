package middlewares

import (
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// AccessLog writes one completion record per request through the pwruntime
// request logger. Response status and size are read from the ResponseTracker
// installed by Track.
func AccessLog() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			status, bytes := http.StatusOK, int64(0)
			if tracker, ok := w.(*ResponseTracker); ok {
				status, bytes = tracker.Status(), tracker.BytesWritten()
			}
			pwruntime.Logger(r.Context()).InfoContext(r.Context(), "request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", bytes,
				"duration", time.Since(start),
			)
		})
	}
}
