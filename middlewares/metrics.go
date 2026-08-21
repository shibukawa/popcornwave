package middlewares

import (
	"net/http"
	"time"

	"github.com/shibukawa/popcornweb/contrib/otel"
	"github.com/shibukawa/popcornweb/pwruntime"
)

// Metrics records the http.server instruments of one request.
//
// It is a frame of its own rather than a line inside the tracing frame, because
// the two are installed independently: a deployment sampling one trace in a
// thousand still counts every request, and a process with tracing off entirely
// is the configuration this exists for. Nothing here reads whether a span is
// recording.
//
// A nil instrument set installs no frame at all, so the feature off costs
// nothing rather than one branch per request.
func Metrics(metrics *pwruntime.Metrics) Middleware {
	if metrics == nil || metrics.RequestDuration == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	// The concurrency attributes have tiny cardinality — a handful of methods
	// against two schemes — and are identical for every request of one pair,
	// so they are built once here and looked up rather than allocated twice
	// per request. The map is never written again, so the lookup needs no lock.
	type methodScheme struct{ method, scheme string }
	activeFor := map[methodScheme][]otel.Attribute{}
	for _, method := range []string{
		"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS",
		"TRACE", "_OTHER",
	} {
		for _, scheme := range []string{"http", "https"} {
			activeFor[methodScheme{method, scheme}] = []otel.Attribute{
				otel.String("http.request.method", method),
				otel.String("url.scheme", scheme),
			}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			// Both labels are bounded before they reach an instrument: the method
			// is folded onto the semconv verb set and the scheme onto {http,https},
			// so a client cannot mint unbounded metric series with arbitrary method
			// tokens or a crafted absolute-form scheme.
			method := pwruntime.NormalizeHTTPMethod(r.Method)
			scheme := pwruntime.NormalizeScheme(requestScheme(r))
			// Concurrency is counted around the whole chain, so a request still
			// running is still counted; the decrement is deferred rather than
			// written after the call because a panic must not leak a count.
			// The normalized pairs are the whole universe the seed above
			// enumerates, so the lookup always answers.
			active := activeFor[methodScheme{method, scheme}]
			metrics.ActiveRequests.Add(r.Context(), 1, active...)
			rw, ok := w.(*ResponseTracker)
			if !ok {
				rw = &ResponseTracker{ResponseWriter: w}
			}
			defer func() {
				metrics.ActiveRequests.Add(r.Context(), -1, active...)
				panicked := recover()
				status := rw.Status()
				if panicked != nil {
					status = http.StatusInternalServerError
				}
				attributes := make([]otel.Attribute, 0, 4)
				attributes = append(attributes,
					otel.String("http.request.method", method),
					otel.String("url.scheme", scheme),
					otel.Int64("http.response.status_code", int64(status)),
				)
				// Absent rather than raw: a request that matched no route
				// carries no route attribute, because the path is the unbounded
				// value the attribute exists to avoid.
				if route := rw.Route(); route != "" {
					attributes = append(attributes, otel.String("http.route", route))
				}
				metrics.RequestDuration.Record(r.Context(), time.Since(started).Seconds(), attributes...)
				metrics.ResponseBodySize.Record(r.Context(), float64(rw.BytesWritten()), attributes...)
				// A chunked request declares no length, and recording zero for
				// one would report a body that was not empty as empty.
				if r.ContentLength > 0 {
					metrics.RequestBodySize.Record(r.Context(), float64(r.ContentLength), attributes...)
				}
				if panicked != nil {
					panic(panicked)
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// requestScheme answers what the client used, falling back to the listener.
func requestScheme(r *http.Request) string {
	if r.URL != nil && r.URL.Scheme != "" {
		return r.URL.Scheme
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
