//go:build !tinygo

package pw

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// notifyShutdownSignals derives a context canceled by an interrupt or SIGTERM,
// so Run drains in-flight requests before closing resources.
func notifyShutdownSignals(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}
