//go:build tinygo

package pw

import "context"

// notifyShutdownSignals installs no signal handler under TinyGo.
//
// TinyGo's os/signal registers a handler that replaces the default disposition
// but never delivers anything to the channel, so signal.NotifyContext would
// leave the process unkillable by Ctrl+C or SIGTERM. Leaving the signals alone
// keeps the default terminate behavior; the caller's context still drives
// graceful shutdown when the application cancels it. Verified with TinyGo
// 0.41.1 on darwin/arm64.
func notifyShutdownSignals(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}
