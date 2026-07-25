package middlewares

import (
	"context"
	"net/http"
	"time"
)

// RequestTimeout installs a request context deadline. A timeout of zero or less
// returns a pass-through middleware.
func RequestTimeout(timeout time.Duration) Middleware {
	if timeout <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
