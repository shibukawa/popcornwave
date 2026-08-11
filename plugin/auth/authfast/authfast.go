// Package authfast serves popcornwave/plugin/auth over the fasthttp transport.
//
// It is the second half of a pair, and it is deliberately small. Everything an
// authentication decision rests on — the configuration, the OIDC client, the
// passkey ceremonies, the bearer verifier, the account admission, the session
// rotation, and every rule about which failure gets which status — lives in
// plugin/auth and is reached through auth.Exchange. What is here is the reader
// of a fasthttp request value and the wiring that positions the frames.
//
// # Why this is not an extension
//
// The net/http half installs itself: importing plugin/auth registers a
// framework extension and the chain picks it up. This transport has no
// extension registry, because pwfast.Middlewares takes what it needs as
// arguments rather than reading a process-wide list. So an application asks for
// authentication by name:
//
//	options, err := authfast.Setup(ctx)
//	if err != nil {
//	    return err
//	}
//	handler, err := pwfast.Middlewares(mux, options.Apply(pwfast.RuntimeOptions{
//	    Session: manager,
//	}))
//
// That is more explicit and it is also the honest shape: a chain assembled from
// arguments cannot silently gain a frame because something was imported.
//
// # What it still needs from the other runtime
//
// Configuration binding has not moved to a shared package yet, so plugin/auth
// reads the settings whichever runtime parsed them published, and this package
// links plugin/auth and therefore pw. requirement:alternate-http-backend-readiness
// settles the destination as a clean split and calls the mixed shape a
// legitimate intermediate; this is that intermediate, and what closes it is the
// configuration layer moving, not anything here.
package authfast

import (
	"context"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pwfast"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Options are what an application adds to pwfast.RuntimeOptions to serve
// authentication on this transport.
//
// It is two fields because the framework chain already owns one of the two
// positions: the guard sits at SlotGuard and pwfast.Middlewares installs it from
// a policy, so this supplies the policy rather than a second guard. The
// authentication frame has no such slot of its own and travels as an extra.
type Options struct {
	// Frames are the authentication frames, positioned by their own slots.
	Frames []pwfast.Frame
	// Guard is the resolved path-protection policy.
	Guard pwfast.GuardPolicy
}

// Apply adds this package's frames and guard policy to runtime options.
//
// It appends rather than replaces, so an application that installs frames of
// its own keeps them, and it leaves an already-set guard policy alone: a
// deployment that wrote its own protection meant it.
func (o Options) Apply(options pwfast.RuntimeOptions) pwfast.RuntimeOptions {
	options.Extra = append(options.Extra, o.Frames...)
	if options.Guard.Protected == nil {
		options.Guard = o.Guard
	}
	return options
}

// Setup builds the authentication runtime from the resolved configuration and
// returns what the chain needs to serve it.
//
// A deployment with auth.enabled off gets the zero value, which installs no
// frame and protects nothing — absent rather than inert, because a guard that
// silently admits everything is a control that looks installed.
//
// It replaces whatever authentication runtime the process had, exactly as the
// net/http extension's setup does, so one process calls one of them once.
func Setup(ctx context.Context) (Options, error) {
	step, err := auth.Setup(ctx)
	if err != nil || step == nil {
		return Options{}, err
	}
	return options(step), nil
}

// Installed returns what the chain needs from the authentication runtime this
// process already built, for a deployment whose startup ran on the other
// runtime.
//
// It is the entry point for a mixed process — one net/http listener and one
// fasthttp listener over the same application — and it is not the same as
// calling Setup twice. Two setups would open two sets of ceremony storage and
// leave the first sweep running over records the second no longer knows about;
// this shares one runtime, which is what a deployment serving two transports
// actually has. A process with no authentication runtime gets the zero value,
// which installs no frame and protects nothing.
func Installed() Options {
	step := auth.Endpoints()
	if step == nil {
		return Options{}
	}
	return options(step)
}

func options(step auth.Step) Options {
	return Options{
		Frames: []pwfast.Frame{{
			Slot:       pwruntime.SlotAuthentication,
			Name:       "auth.endpoints",
			Middleware: Frame(step),
		}},
		Guard: guardPolicy(),
	}
}

// Frame wraps a neutral authentication step in this transport's middleware
// shape.
//
// The wrapping is the whole difference between the transports: the other half
// hands a derived request to the next handler, and this one writes into the
// request value it already has and calls next with the same value.
func Frame(step auth.Step) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			step(newExchange(r), func() { next(r) })
		}
	}
}

// GuardPolicy returns the resolved path-protection policy of the running
// deployment, for an application assembling a chain by hand.
func GuardPolicy() pwfast.GuardPolicy { return guardPolicy() }

func guardPolicy() pwfast.GuardPolicy {
	rules := auth.Protection()
	return pwfast.GuardPolicy{
		Protected:   rules.Protected,
		LoginURL:    rules.LoginURL,
		Redirect:    rules.Redirect,
		BearerRealm: rules.BearerRealm,
	}
}

// EstablishSession creates the login session of an account this application
// authenticated through a flow the framework does not own.
//
// It authenticates nobody: the caller has already decided that this request
// belongs to this account. See auth.EstablishSession for the whole of what it
// means.
func EstablishSession(r *fasthttp.RequestCtx, data auth.SessionData, method string) error {
	return auth.EstablishSessionOn(newExchange(r), data, method)
}

// Hint returns the sign-in hint of the request, for a login screen to render.
func Hint(r *fasthttp.RequestCtx) (auth.SignInHint, bool) {
	return auth.HintOn(newExchange(r))
}

// Ensure wraps a page route: a request whose proof is older than the
// requirement is redirected into re-proof and returns to this operation.
func Ensure(handler fasthttp.RequestHandler, requirement auth.Requirement) fasthttp.RequestHandler {
	return ensure(handler, requirement, false)
}

// EnsureAPI wraps an API route: an unmet requirement answers 401 with a problem
// document naming the window, so a client can start the step-up itself.
//
// It is a separate function rather than an option with a default for the reason
// auth.EnsureAPI is: answering a fetch with a redirect is followed
// transparently, so the client receives 200 and a login page instead of an
// error, and a route that forgot to pass an option would take the silent
// failure.
func EnsureAPI(handler fasthttp.RequestHandler, requirement auth.Requirement) fasthttp.RequestHandler {
	return ensure(handler, requirement, true)
}

func ensure(handler fasthttp.RequestHandler, requirement auth.Requirement, api bool) fasthttp.RequestHandler {
	return func(r *fasthttp.RequestCtx) {
		if auth.AdmitAssurance(newExchange(r), requirement, api) {
			handler(r)
		}
	}
}

// IsRecent reports whether the requirement is met and writes nothing. Use it
// with Challenge when the window depends on something only the handler knows.
func IsRecent(r *fasthttp.RequestCtx, requirement auth.Requirement) bool {
	return auth.IsRecentOn(newExchange(r), requirement)
}

// Challenge writes the response an unmet requirement produces and starts the
// step-up. Pass api = true from a route whose caller is not a browser.
func Challenge(r *fasthttp.RequestCtx, requirement auth.Requirement, api bool) {
	auth.ChallengeOn(newExchange(r), requirement, api)
}

// Contribute builds the authentication runtime and returns what it adds to a
// chain, in the shape pwfast.WithSetup takes:
//
//	pwfast.Run(ctx, handler, pwfast.WithSetup(authfast.Contribute))
//
// It exists because the other transport gains these frames from a blank import
// and this one cannot: a chain assembled from arguments does not silently gain
// a frame, so an application names them. One line rather than three is what
// keeps that from being a reason to skip it.
func Contribute(ctx context.Context) (func(pwfast.RuntimeOptions) pwfast.RuntimeOptions, error) {
	options, err := Setup(ctx)
	if err != nil {
		return nil, err
	}
	return options.Apply, nil
}
