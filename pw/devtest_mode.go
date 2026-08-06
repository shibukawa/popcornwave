//go:build !pwdev

package pw

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// A release build serves no test data endpoints and links none of the seeding
// machinery. The absence is structural rather than conditional: /_pw/test/ is
// answered 404 by the closed reserved namespace, exactly like any other path
// nothing claims.
func developmentTestEndpoints(next http.Handler, _ MiddlewareConfig, _ pwruntime.Resources) http.Handler {
	return next
}
