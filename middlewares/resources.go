package middlewares

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// InjectResources publishes the process runtime resources, such as loaded
// configuration, the logger, and the database pool, on every request context.
func InjectResources(resources pwruntime.Resources) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(pwruntime.WithResources(r.Context(), resources)))
		})
	}
}
