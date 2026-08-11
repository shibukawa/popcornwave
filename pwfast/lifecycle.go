package pwfast

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/shibukawa/popcornwave/internal/requestorigin"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Slot orders every frame of the request chain, and the numbers are the shared
// leaf's, so a chain assembled here runs in the order the other transport's
// does.
//
// That the numbers are shared is the point rather than a tidiness: a chain
// whose frames run in a different order on the second transport is a different
// application. A guard running after the session on one and before it on the
// other would authorize differently, and nothing about either response would
// say so.
type Slot = pwruntime.Slot

const (
	SlotTracing          = pwruntime.SlotTracing
	SlotResources        = pwruntime.SlotResources
	SlotClientAddress    = pwruntime.SlotClientAddress
	SlotRequestID        = pwruntime.SlotRequestID
	SlotAccessLog        = pwruntime.SlotAccessLog
	SlotRecover          = pwruntime.SlotRecover
	SlotRateLimitProcess = pwruntime.SlotRateLimitProcess
	SlotSecurityHeaders  = pwruntime.SlotSecurityHeaders
	SlotRequestTimeout   = pwruntime.SlotRequestTimeout
	SlotMaxRequestBody   = pwruntime.SlotMaxRequestBody
	SlotPublicAssets     = pwruntime.SlotPublicAssets
	SlotOperational      = pwruntime.SlotOperational
	SlotStorage          = pwruntime.SlotStorage
	SlotSession          = pwruntime.SlotSession
	SlotAuthentication   = pwruntime.SlotAuthentication
	SlotRateLimit        = pwruntime.SlotRateLimit
	SlotCSRF             = pwruntime.SlotCSRF
	SlotGuard            = pwruntime.SlotGuard
	SlotAPIDoc           = pwruntime.SlotAPIDoc
)

// Frame is one positioned step of the chain.
type Frame = pwruntime.Frame[fasthttp.RequestHandler]

// Compose wraps handler in frames, outermost first by slot, through the shared
// ordering rule.
func Compose(handler fasthttp.RequestHandler, frames ...Frame) fasthttp.RequestHandler {
	return pwruntime.Compose(handler, frames)
}

// ResolveClientAddress records the caller every downstream bound counts
// against.
//
// The walk through the forwarded chain is the shared one, so this transport and
// the other name the same caller for the same request. Getting that wrong is
// quiet: an unresolved address counts the proxy, and every rate limit and live
// bound then applies to one address for every visitor.
func ResolveClientAddress(trustedProxies []*net.IPNet) Middleware {
	proxies := requestorigin.FromNetworks(trustedProxies)
	var undeclared sync.Once
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			if proxies.Empty() && forwardedBy(r) {
				undeclared.Do(func() {
					pwruntime.ReadLogger(r).Log(r, pwruntime.LevelWarn,
						"request carries forwarding headers but no trusted proxy is configured")
				})
			}
			pwruntime.StoreClientAddress(r, proxies.ClientAddressOf(
				r.RemoteIP().String(), forwardedFor(r)))
			next(r)
		}
	}
}

func forwardedBy(r *fasthttp.RequestCtx) bool {
	return len(r.Request.Header.Peek("X-Forwarded-For")) > 0 ||
		len(r.Request.Header.Peek("X-Forwarded-Proto")) > 0
}

// forwardedFor collects every X-Forwarded-For line, because the header
// legitimately repeats and only reading the first would drop hops.
func forwardedFor(r *fasthttp.RequestCtx) []string {
	var lines []string
	r.Request.Header.VisitAll(func(name, value []byte) {
		if string(name) == "X-Forwarded-For" || string(name) == "x-forwarded-for" {
			lines = append(lines, string(value))
		}
	})
	return lines
}

// RuntimeOptions are what Middlewares needs that configuration does not carry.
type RuntimeOptions struct {
	// Resources is the capsule every request is served with. A zero value
	// serves requests with the process defaults.
	Resources pwruntime.Resources
	// TrustedProxies are the networks whose forwarding headers this deployment
	// reads.
	TrustedProxies []*net.IPNet
	// Extra frames are installed alongside the framework's, positioned by their
	// own slots.
	Extra []Frame
}

// Middlewares builds the framework chain around handler.
//
// It is the second transport's counterpart to pw.Middlewares, and it is
// deliberately smaller than that one. The other builds the whole chain and also
// performs framework initialization — configuration parsing, database startup,
// observability, the validations that must fail before a port is bound. None of
// that is transport-shaped and none of it is duplicated here: a deployment runs
// it once, on whichever runtime owns startup, and what this assembles is the
// request path.
//
// # What is not here yet
//
// The session, CSRF, authentication and guard frames, the public asset frame,
// the operational and documentation endpoints, and the extension chain. Each is
// absent rather than stubbed, so a build that needs one fails to name it rather
// than serving requests with a frame that silently does nothing — which for the
// session and CSRF frames would be a security control that looks installed.
func Middlewares(handler fasthttp.RequestHandler, options RuntimeOptions) (fasthttp.RequestHandler, error) {
	if handler == nil {
		return nil, errors.New("popcornwave: nil handler")
	}
	settings, ok := pwruntime.ResolvedChainSettings()
	if !ok {
		// Composing from zero values would produce a chain with no recovery
		// frame, no request ID and no security headers, which serves requests
		// and looks like a chain. Refusing names the actual problem.
		return nil, errors.New("popcornwave: no chain settings published; the runtime that binds configuration has not run")
	}
	trusted := options.TrustedProxies
	if len(trusted) == 0 {
		if compiled, err := compileTrustedProxies(settings.TrustedProxies); err == nil {
			trusted = compiled
		} else {
			return nil, err
		}
	}

	frames := []Frame{
		{Slot: SlotResources, Name: "resources", Middleware: InjectResources(options.Resources)},
		{Slot: SlotClientAddress, Name: "client_address", Middleware: ResolveClientAddress(trusted)},
	}
	if settings.Tracing {
		// Tracing wraps every positioned frame, so the request root span covers
		// the whole chain and every record taken inside it correlates. It is
		// omitted when nothing exports, for the same reason as on the other
		// half: an unsampled span is pure cost.
		frames = append(frames, Frame{Slot: SlotTracing, Name: "otel", Middleware: Otel()})
	}
	if settings.RequestID {
		frames = append(frames, Frame{Slot: SlotRequestID, Name: "request_id", Middleware: RequestID()})
	}
	if settings.AccessLog {
		frames = append(frames, Frame{Slot: SlotAccessLog, Name: "access_log", Middleware: AccessLog()})
	}
	if settings.Recovery {
		frames = append(frames, Frame{Slot: SlotRecover, Name: "recover", Middleware: Recover(writePanicProblem)})
	}
	if settings.SecurityHeaders.Enabled {
		headers, err := SecurityHeaders(settings.SecurityHeaders, WithTrustedProxies(trusted))
		if err != nil {
			return nil, err
		}
		frames = append(frames, Frame{Slot: SlotSecurityHeaders, Name: "security_headers", Middleware: headers})
	}
	if settings.RequestTimeout > 0 {
		frames = append(frames, Frame{Slot: SlotRequestTimeout, Name: "request_timeout", Middleware: RequestTimeout(settings.RequestTimeout)})
	}
	if settings.MaxRequestBody > 0 {
		frames = append(frames, Frame{Slot: SlotMaxRequestBody, Name: "max_request_body", Middleware: MaxRequestBody(settings.MaxRequestBody)})
	}
	frames = append(frames, options.Extra...)
	return Compose(handler, frames...), nil
}

// writePanicProblem answers a recovered panic with the framework problem
// document, which is what the other half answers with.
func writePanicProblem(r *fasthttp.RequestCtx, err error) {
	pwruntime.ReadLogger(r).Log(r, pwruntime.LevelError, "recovered panic",
		pwruntime.String("error", err.Error()))
	r.Response.ResetBody()
	WriteProblem(r, InternalServerError(err))
}

// Run serves handler until ctx is cancelled, then shuts down.
//
// The listener is this function's, unlike pw.Run, which also owns configuration
// parsing and the framework actions. Startup belongs to whichever runtime binds
// the configuration; this owns the port.
func Run(ctx context.Context, address string, handler fasthttp.RequestHandler, options RuntimeOptions) error {
	if ctx == nil {
		return errors.New("popcornwave: nil context")
	}
	wrapped, err := Middlewares(handler, options)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return Serve(ctx, listener, wrapped)
}

// Serve runs an already-built handler on a listener the caller owns, which is
// what a test and an application embedding this framework both need.
func Serve(ctx context.Context, listener net.Listener, handler fasthttp.RequestHandler) error {
	server := &fasthttp.Server{Handler: handler}
	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
		// Shutdown closes the listener and waits for in-flight requests, so a
		// cancelled context ends the process without cutting a response in half.
		if err := server.Shutdown(); err != nil {
			return err
		}
		<-failed
		return nil
	}
}

// compileTrustedProxies turns configured addresses and CIDR blocks into the
// trust set, naming the offending value when one does not parse.
func compileTrustedProxies(values []string) ([]*net.IPNet, error) {
	proxies, err := requestorigin.Compile(values)
	if err != nil {
		return nil, err
	}
	return proxies.Networks(), nil
}
