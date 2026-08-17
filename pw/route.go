package pw

import (
	"net/http"

	"github.com/shibukawa/popcornwave/middlewares"
)

// SetRoute records which route this response belongs to, so a metric is keyed by
// the pattern rather than by the path.
//
// The framework calls it from every response writer it owns, so a generated page
// or API handler needs nothing. A hand-written handler that writes its own
// response and wants its metrics grouped calls it once, with the request the mux
// handed it.
//
// It exists because the pattern cannot be read where the metric is recorded:
// net/http sets Request.Pattern on the copy the mux passes to the handler, and
// the frame that records the metric wraps the mux and never sees that copy. The
// value therefore travels back up on the response writer, which is the one thing
// the handler and the frame both hold.
//
// A request that matched no pattern records no route, and never the raw path: an
// unbounded attribute is what the route attribute exists to avoid.
func SetRoute(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		return
	}
	middlewares.SetRoute(w, r.Pattern)
}
