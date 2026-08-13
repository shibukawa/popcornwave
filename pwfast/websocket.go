package pwfast

import (
	"github.com/shibukawa/popcornwave/pwconfig"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Socket is the module's own typed WebSocket connection, the same declaration
// pw aliases, so the value a callback receives is one type across the pair.
type Socket[In, Out any] = fasthttpbind.Socket[In, Out]

// SocketOptions configures one socket. A zero field takes the process default,
// and nothing reaches the connection as zero.
type SocketOptions = fasthttpbind.SocketOptions

// SetSocketDefaults installs the process-wide socket options. It is shared with
// the net/http runtime, so installing them once covers both.
func SetSocketDefaults(opts SocketOptions) {
	pwruntime.SetSocketOriginPolicy(opts.CheckOrigin)
	fasthttpbind.SetSocketDefaults(opts)
}

// SocketDefaults returns the effective process defaults.
func SocketDefaults() SocketOptions { return fasthttpbind.SocketDefaults() }

// WebSocket upgrades the request, runs fn against a typed socket, and closes
// the socket when fn returns.
//
// The return value is the handshake error and nothing else; a non-nil value
// means the refusal response has already been written as a problem document.
// fn's own error is post-commit and reaches SetStreamErrorHandler.
//
// fn runs after the handler has returned, from the hijacked connection, so it
// must not read r: everything it needs is captured before WebSocket returns.
// This transport closes the connection when fn returns, which is what the
// callback shape wants.
//
// The origin check is this framework's, resolved through the same declared
// proxies and trusted origins the net/http half reads, so one deployment
// answers one way whichever runtime serves it.
func WebSocket[In, Out any](r *fasthttp.RequestCtx, fn func(*Socket[In, Out]) error) error {
	return WebSocketWith[In, Out](r, SocketOptions{}, fn)
}

// WebSocketWith is WebSocket with per-call options.
func WebSocketWith[In, Out any](r *fasthttp.RequestCtx, opts SocketOptions, fn func(*Socket[In, Out]) error) error {
	if opts.CheckOrigin == nil && r != nil {
		opts.CheckOrigin = pwruntime.SocketOriginCheck(pwruntime.SocketHandshake{
			TLS:            r.IsTLS(),
			Host:           string(r.Host()),
			RemoteAddress:  r.RemoteIP().String(),
			ForwardedProto: string(r.Request.Header.Peek("X-Forwarded-Proto")),
		}, pwconfig.Development())
	}
	return fasthttpbind.WebSocketWith[In, Out](r, opts, fn)
}
