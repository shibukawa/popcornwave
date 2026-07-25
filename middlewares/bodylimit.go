package middlewares

import "net/http"

// MaxRequestBody limits downstream reads from the request body. A limit of zero
// or less disables the limit and returns a pass-through middleware.
func MaxRequestBody(limit int64) Middleware {
	if limit <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
