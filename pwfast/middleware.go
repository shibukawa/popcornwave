package pwfast

import (
	"github.com/shibukawa/popcornwave/internal/requestid"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Middleware is the chain shape on this transport.
//
// It is the same idea as the other half's func(http.Handler) http.Handler and
// not the same type, which is the point decision:backend-specific-middleware
// already settled: an application's own middleware is written per backend,
// because a frame that wraps a handler necessarily names the handler.
//
// What is portable is what a frame does rather than how it wraps. A frame that
// reads a header, decides something, and records it for the rest of the chain
// has one implementation of the deciding and two of the reading and recording.
type Middleware = func(fasthttp.RequestHandler) fasthttp.RequestHandler

// Chain wraps handler in the given middleware, outermost first.
//
// The order is the reading order: Chain(h, a, b) runs a, then b, then h. That
// is the order pw.Slot numbers describe, so a chain assembled from the same
// list on either transport runs in the same order.
func Chain(handler fasthttp.RequestHandler, middleware ...Middleware) fasthttp.RequestHandler {
	for index := len(middleware) - 1; index >= 0; index-- {
		if middleware[index] != nil {
			handler = middleware[index](handler)
		}
	}
	return handler
}

// RequestIDOption configures RequestID.
type RequestIDOption func(*requestIDConfig)

type requestIDConfig struct {
	header   string
	generate func() string
}

// WithRequestIDHeader replaces the default X-Request-ID header name.
func WithRequestIDHeader(name string) RequestIDOption {
	return func(c *requestIDConfig) {
		if name != "" {
			c.header = name
		}
	}
}

// WithRequestIDGenerator replaces the generator used when the client did not
// send a usable request ID.
func WithRequestIDGenerator(generate func() string) RequestIDOption {
	return func(c *requestIDConfig) {
		if generate != nil {
			c.generate = generate
		}
	}
}

// RequestID validates or creates a request ID, echoes it on the response, and
// records it as a log attribute every later record carries.
//
// The check on a client-supplied value is the shared one, because it is a
// security check rather than a convenience: the value arrives from the client
// and leaves in a response header, so a second copy of the rule would be a
// second chance to get it wrong.
//
// Where this differs from the other half is only the recording. That one
// derives a context per frame and hands it on; here the request value is the
// context, so the attribute is written into it in place. Everything that reads
// it afterwards is unchanged, because the request value answers Value from the
// same store.
func RequestID(options ...RequestIDOption) Middleware {
	config := requestIDConfig{header: requestid.DefaultHeader, generate: requestid.Sequential}
	for _, option := range options {
		option(&config)
	}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			id := string(r.Request.Header.Peek(config.header))
			if !requestid.Valid(id) {
				id = config.generate()
			}
			r.Response.Header.Set(config.header, id)
			pwruntime.StoreLogAttributes(r, pwruntime.String("request_id", id))
			next(r)
		}
	}
}
