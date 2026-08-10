package pwfast

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"sync"

	"github.com/shibukawa/popcornwave/internal/apidoc"
	"github.com/shibukawa/popcornwave/internal/requestorigin"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
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
	// Tracing installs the request root span. It is a runtime option rather
	// than a setting because whether a span has anywhere to go is decided by
	// the exporter this process built, not by a configuration key.
	Tracing bool
	// PublicFS is the embedded public tree. It is supplied rather than read
	// from configuration because an embed is a compile-time fact of the
	// application binary rather than something a settings file can name.
	PublicFS fs.FS
	// Session is the manager the session frame resolves through, or nil where a
	// deployment disabled session storage. It is supplied rather than built
	// here because building one is startup work — a registry, a keyring, a
	// backend — and startup belongs to whichever runtime binds configuration.
	Session *session.Manager
	// SessionCookie and SessionSameSite describe the cookie the CSRF companion
	// is issued beside, so the two travel with the same policy.
	SessionCookie   session.CookieOptions
	SessionSameSite http.SameSite
	// Guard is the authorization policy. It is supplied rather than derived
	// because deciding which paths are protected belongs to an authentication
	// plugin, and this half applies what that plugin resolved.
	Guard GuardPolicy
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
// The authentication frames — the login, callback and logout endpoints of an
// identity provider — and no extension registry. Each is absent rather than stubbed, so a
// build that needs one fails to name it rather than serving requests with a
// frame that silently does nothing — which for a guard would be an
// authorization check that looks installed.
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
	if options.PublicFS != nil && settings.Public.Enabled {
		assets, err := PublicAssets(settings.Public, options.PublicFS)
		if err != nil {
			return nil, err
		}
		frames = append(frames, Frame{Slot: SlotPublicAssets, Name: "public_assets", Middleware: assets})
	}
	if options.Session != nil {
		frames = append(frames, Frame{Slot: SlotSession, Name: "session",
			Middleware: Session(options.Session, nil)})
	}
	if settings.CSRF.Enabled {
		// The check is built even when the session frame is absent, and refuses
		// rather than passing: with no session there is nothing a request could
		// present that would be valid, and letting it through would be the one
		// failure direction this check must not have.
		check, err := CSRF(settings.CSRF, options.SessionCookie, options.SessionSameSite, nil, trusted)
		if err != nil {
			return nil, err
		}
		frames = append(frames, Frame{Slot: SlotCSRF, Name: "csrf", Middleware: check})
	}
	frames = append(frames, Frame{Slot: SlotOperational, Name: "operational",
		Middleware: OperationalEndpoints(settings.Health, settings.Readiness, options.Resources)})
	frames = append(frames, Frame{Slot: SlotAPIDoc, Name: "apidoc",
		Middleware: DocumentationEndpoints(settings.OpenAPI, settings.APIDoc, settings.APIDocPath)})
	if options.Guard.Protected != nil {
		frames = append(frames, Frame{Slot: SlotGuard, Name: "guard", Middleware: Guard(options.Guard)})
	}
	if options.Tracing {
		// Outermost of everything positioned, so the request root span covers
		// the whole chain and every record taken inside it correlates. It is
		// opt-in because an unsampled span with nowhere to export is pure cost.
		frames = append(frames, Frame{Slot: SlotTracing, Name: "otel", Middleware: Otel()})
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

// OperationalEndpoints answers the liveness and readiness probes above
// everything that authenticates.
//
// The probes reveal only status and are reachable by anything that can reach
// the port, which is what a liveness probe needs and what keeps a dependency
// outage from turning into a restart loop. Readiness is the shared probe, so
// the same process reports the same readiness whichever transport asked.
//
// A path left empty installs nothing for it, which is how a deployment turns
// one off.
func OperationalEndpoints(health, readiness string, resources pwruntime.Resources) Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		if health == "" && readiness == "" {
			return next
		}
		return func(r *fasthttp.RequestCtx) {
			switch path := string(r.Path()); {
			case health != "" && path == health:
				writeOperationalStatus(r, true)
			case readiness != "" && path == readiness:
				writeOperationalStatus(r, pwruntime.DatabasesReady(r, resources))
			default:
				next(r)
			}
		}
	}
}

// writeOperationalStatus answers a probe.
//
// Only GET and HEAD are answered, because a probe endpoint that accepts any
// method is one an arbitrary caller can POST to, and the reply says nothing but
// costs a database round trip on the readiness path.
func writeOperationalStatus(r *fasthttp.RequestCtx, healthy bool) {
	if !operationalMethod(r) {
		return
	}
	method := string(r.Method())
	r.Response.Header.Set("Cache-Control", "no-store")
	r.Response.Header.SetContentType("text/plain; charset=utf-8")
	status, body := fasthttp.StatusOK, "ok\n"
	if !healthy {
		status, body = fasthttp.StatusServiceUnavailable, "unavailable\n"
	}
	r.SetStatusCode(status)
	if method != fasthttp.MethodHead {
		_, _ = r.WriteString(body)
	}
}

// DocumentationEndpoints answers the OpenAPI document and the UI over it.
//
// Unlike the probes it belongs beneath whatever protects the routes it
// describes: an API description is a map of the whole application surface, so
// reaching it costs a session where the configuration says so. That is why its
// slot is below the guard rather than beside the operational frame, and why
// this returns a frame rather than being folded into the one above it.
//
// A configuration naming neither returns the handler unchanged, so the common
// case adds nothing to the chain.
func DocumentationEndpoints(openAPIPath, docKind, docPath string) Middleware {
	page, hasPage := apidoc.Build(docKind, openAPIPath)
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		if openAPIPath == "" && !hasPage {
			return next
		}
		return func(r *fasthttp.RequestCtx) {
			switch path := string(r.Path()); {
			case openAPIPath != "" && path == openAPIPath:
				if !operationalMethod(r) {
					return
				}
				if string(r.Method()) == fasthttp.MethodHead {
					// The document is assembled either way, because its length
					// is the answer a HEAD is asking for.
					OpenAPIJSON(r)
					r.Response.ResetBody()
					return
				}
				OpenAPIJSON(r)
			case hasPage && docPath != "" && path == docPath:
				if !operationalMethod(r) {
					return
				}
				writeAPIDocPage(r, page)
			default:
				next(r)
			}
		}
	}
}

// writeAPIDocPage sends the composed page under the policy it needs.
//
// The policy replaces the application's rather than widening it, and only where
// one is already set: the security header frame wraps this endpoint, so the
// configured policy is on the response while it is still uncommitted, and
// widening the configured value instead would carry the CDN and inline
// allowances into every response the application sends.
func writeAPIDocPage(r *fasthttp.RequestCtx, page apidoc.Page) {
	if page.CSP != "" {
		for _, name := range apidoc.RelaxedPolicyNames {
			if len(r.Response.Header.Peek(name)) > 0 {
				r.Response.Header.Set(name, page.CSP)
			}
		}
	}
	r.Response.Header.SetContentType("text/html; charset=utf-8")
	r.SetStatusCode(fasthttp.StatusOK)
	if string(r.Method()) != fasthttp.MethodHead {
		_, _ = r.WriteString(page.HTML)
	}
}

// operationalMethod refuses a method these endpoints do not answer, reporting
// whether the caller may continue.
func operationalMethod(r *fasthttp.RequestCtx) bool {
	if method := string(r.Method()); method == fasthttp.MethodGet || method == fasthttp.MethodHead {
		return true
	}
	r.Response.Header.Set("Allow", "GET, HEAD")
	r.SetStatusCode(fasthttp.StatusMethodNotAllowed)
	return false
}
