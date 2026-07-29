package petitweb

// This file only adapts the shared middleware in
// github.com/shibukawa/popcornwave/middlewares to the classic App API. New
// middleware belongs in that package.

import (
	"context"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// SecurityHeadersConfig contains browser security response headers.
type SecurityHeadersConfig = middlewares.SecurityHeadersConfig

// HSTSConfig controls Strict-Transport-Security on direct HTTPS requests.
type HSTSConfig = middlewares.HSTSConfig

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig { return middlewares.DefaultSecurityHeaders() }

// RequestID validates or creates a request ID and exposes it through context.
// The zero Logger is accepted and resolves to the one installed on the request.
func RequestID(header string, logger Logger) Middleware {
	return Middleware(middlewares.RequestID(
		middlewares.WithRequestIDHeader(header),
		middlewares.WithRequestIDGenerator(middlewares.RandomRequestID),
		middlewares.WithRequestIDContext(func(ctx context.Context, id string) context.Context {
			bound := logger
			if !bound.Enabled(pwruntime.LevelError) {
				bound = pwruntime.ReadLogger(ctx)
			}
			return withRequestValues(ctx, requestValues{requestID: id, logger: bound.With(pwruntime.String("request_id", id))})
		}),
	))
}

// Recover converts a panic into a safe negotiated error response.
func Recover(handler ErrorHandler) Middleware {
	return Middleware(middlewares.Recover(handler.WriteError))
}

// MaxRequestBody limits downstream reads from the request body.
func MaxRequestBody(bytes int64) Middleware {
	if bytes <= 0 {
		panic("petitweb: request body limit must be positive")
	}
	return Middleware(middlewares.MaxRequestBody(bytes))
}

// SecurityHeaders sets policy headers before downstream response commitment.
// Strict-Transport-Security is limited to direct HTTPS connections.
func SecurityHeaders(config SecurityHeadersConfig) (Middleware, error) {
	middleware, err := middlewares.SecurityHeaders(config)
	if err != nil {
		return nil, err
	}
	return Middleware(middleware), nil
}
