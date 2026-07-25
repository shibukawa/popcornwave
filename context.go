package petitweb

import (
	"context"
	"log/slog"
)

type contextKey struct{}

type requestValues struct {
	requestID string
	logger    *slog.Logger
}

func withRequestValues(ctx context.Context, values requestValues) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, &values)
}

func readRequestValues(ctx context.Context) (*requestValues, bool) {
	if ctx == nil {
		return nil, false
	}
	values, ok := ctx.Value(contextKey{}).(*requestValues)
	return values, ok && values != nil
}

// ReadRequestID returns the validated request correlation ID.
func ReadRequestID(ctx context.Context) (string, bool) {
	values, ok := readRequestValues(ctx)
	if !ok || values.requestID == "" {
		return "", false
	}
	return values.requestID, true
}

// ReadLogger always returns a non-nil request-aware logger.
func ReadLogger(ctx context.Context) *slog.Logger {
	values, ok := readRequestValues(ctx)
	if !ok || values.logger == nil {
		return slog.Default()
	}
	return values.logger
}
