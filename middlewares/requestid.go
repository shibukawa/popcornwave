package middlewares

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// DefaultRequestIDHeader carries the request correlation ID in both directions.
const DefaultRequestIDHeader = "X-Request-ID"

var requestSequence atomic.Uint64

type requestIDConfig struct {
	header   string
	generate func() string
	bind     func(context.Context, string) context.Context
}

// RequestIDOption configures RequestID.
type RequestIDOption func(*requestIDConfig)

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

// WithRequestIDContext replaces how the accepted request ID is published to the
// request context.
func WithRequestIDContext(bind func(context.Context, string) context.Context) RequestIDOption {
	return func(c *requestIDConfig) {
		if bind != nil {
			c.bind = bind
		}
	}
}

// RequestID validates or creates a request ID, echoes it on the response, and
// exposes it through the request context. By default it binds the ID to the
// pwruntime request logger.
func RequestID(options ...RequestIDOption) Middleware {
	config := requestIDConfig{header: DefaultRequestIDHeader, generate: SequentialRequestID, bind: bindRequestIDLogger}
	for _, option := range options {
		option(&config)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(config.header)
			if !ValidRequestID(id) {
				id = config.generate()
			}
			w.Header().Set(config.header, id)
			next.ServeHTTP(w, r.WithContext(config.bind(r.Context(), id)))
		})
	}
}

// ValidRequestID reports whether a client supplied ID is safe to echo back.
func ValidRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

// SequentialRequestID builds an ID from the clock and a process counter.
func SequentialRequestID() string {
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), requestSequence.Add(1))
}

// RandomRequestID builds an ID from 16 cryptographically random bytes.
func RandomRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "popcornwave-request"
	}
	return hex.EncodeToString(bytes[:])
}

// bindRequestIDLogger records the correlation ID as a stable request
// attribute, so every record taken from the request afterwards carries it
// without a handler passing it along.
func bindRequestIDLogger(ctx context.Context, id string) context.Context {
	return pwruntime.WithLogAttributes(ctx, pwruntime.String("request_id", id))
}
