package pw

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/shibukawa/popcornwave/internal/safeurl"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// Update protocol names.
//
// These are fixed rather than configured. They are contracts between this
// framework and the runtime it ships — the header namespace, the attribute
// prefix, the installed name, and the endpoint prefix all reach the browser as
// one configuration object, and a deployment changing one would be describing a
// framework it is not running.
const (
	// UpdateHeaderPrefix yields Pw-Render, Pw-Manifest, and Pw-Build.
	UpdateHeaderPrefix = "Pw"
	// UpdateAttributePrefix names the boundary attributes generation writes and
	// the placeholder element the render option spells, so one document holds
	// one spelling rather than two.
	//
	// It is the module's default rather than this framework's brand because
	// routetree compiles a page tree's templates without the prefix option, so
	// branding it here would split a document's naming in exactly the way the
	// option exists to prevent. internal/pwgen names the same value.
	UpdateAttributePrefix = "tb"
	// UpdateGlobalName is the browser namespace api:client-update-api installs.
	UpdateGlobalName = "popcornwave"
)

// updateBuildID identifies the binary that rendered a page.
//
// It answers the same question the live delivery stream's version does: was the
// page asking rendered by this build? A page from another one holds client
// state this binary cannot vouch for — a template it does not have, a runtime
// that renders differently — and none of that is visible in a validator.
//
// The two differ on an unstamped binary, and the difference is deliberate. Live
// delivery reports nothing there, which disables its check rather than inventing
// a value that would differ per process and reload every client on every
// restart. An update falls back to the module's per-process identity instead,
// which costs a complete document after a restart and never a wrong delta. A
// frozen screen is worse than a re-transferred page.
var updateBuildID = sync.OnceValue(func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
})

// UpdateBuildID is the identity a rendered page carries and every update
// request is checked against, so a page from another build is answered with a
// complete document and a redraw from one is refused.
//
// It is the effective value rather than the stamp: an unstamped binary has no
// vcs.revision, and the module's per-process identity stands in, which costs a
// complete document after a restart and never a wrong delta. Reading it is what
// a diagnostic and a test both need, since neither can derive it.
func UpdateBuildID() string {
	return updateOptions(Config[HTMLConfig](nil)).RuntimeConfig().Build
}

// updateOptions builds the transport configuration for one process.
//
// Everything the module could decide for itself is decided here instead, so a
// project reads one framework rather than one framework and its dependency.
func updateOptions(config HTMLConfig) htmlupdate.Options {
	return htmlupdate.Options{
		Key:                 []byte(config.Update.ValidatorKey),
		HeaderPrefix:        UpdateHeaderPrefix,
		DataAttributePrefix: UpdateAttributePrefix,
		GlobalName:          UpdateGlobalName,
		PathPrefix:          updatePathPrefix,
		BuildID:             updateBuildID(),
		MaxManifestBytes:    config.Update.MaxManifestBytes,
		CSRFHeaderName:      pwruntime.CSRFHeaderName,
		// The merged asset of requirement:unified-update-runtime is this
		// framework's, so the module serves none and emits no tag of its own.
		CallerOwnsRuntime: true,
		OnFailure:         writeUpdateFailure,
	}
}

// updateHeadNodes contributes the runtime reference and its configuration.
//
// They are contributed at the render call rather than written into the
// scaffolded document shell, because that shell is an application file the
// author owns and may rewrite. A removed script tag would disable every client
// capability with no error, no console message, and no failed request; the one
// shell edit this cannot survive is deleting the head element itself, which
// rule:route-and-template-checks reports.
//
// The configuration travels as an inert escaped meta rather than as attributes
// on the tag, because the runtime is a module script and cannot read its own.
func updateHeadNodes(config HTMLConfig, csrfToken string) []htmlbind.HeadNode {
	if !config.Update.Enabled {
		return nil
	}
	encoded, err := json.Marshal(updateOptions(config).RuntimeConfigFor(csrfToken))
	if err != nil {
		return nil
	}
	return []htmlbind.HeadNode{
		htmlbind.HeadMeta(
			htmlbind.HeadAttr{Name: "name", Value: updateConfigMetaName},
			htmlbind.HeadAttr{Name: "content", Value: string(encoded)},
		),
		htmlbind.HeadScript(
			htmlbind.HeadAttr{Name: "src", Value: RuntimeScriptURL()},
			htmlbind.HeadAttr{Name: "type", Value: "module"},
		),
	}
}

// updateConfigMetaName is what the bootstrap half of the merged runtime looks
// for. It is a contract between this file and updateboot.js.
const updateConfigMetaName = "pw-runtime"

// updatePathPrefix is the reserved namespace every framework endpoint lives
// under, so one routing, caching, and access rule covers the whole surface.
const updatePathPrefix = "/_pw"

// ErrUpdateKeyMissing reports updates enabled with nothing to key validators
// with.
var ErrUpdateKeyMissing = errors.New("popcornwave: html.update.validator_key is required when html.update.enabled is true")

// validateUpdateConfig refuses a configuration that would serve unkeyed
// validators.
//
// An unkeyed digest of low-entropy content lets anyone confirm a guess by
// comparing digests, so this fails startup rather than degrading quietly.
func validateUpdateConfig(config HTMLConfig) error {
	if !config.Update.Enabled {
		return nil
	}
	if config.Update.ValidatorKey == "" {
		return ErrUpdateKeyMissing
	}
	return updateOptions(config).Validate()
}

// writeUpdateFailure routes a refused redraw into the framework error path.
//
// The module would otherwise write plain text, which no application error page
// and no request log would ever see. Version skew is the ordinary case here: a
// page loaded before a deploy asks for a component whose markup has changed,
// gets a 404, and reloads. It is recorded rather than treated as a fault.
//
// A stale build is no longer one of these. Since system:tinybind v0.3.5 a redraw
// is answered at the page's own URL, so a request from another build is not
// refused at all: the caller renders the page it was going to render, which
// costs a reload instead of a refusal followed by one.
func writeUpdateFailure(w http.ResponseWriter, r *http.Request, failure htmlupdate.Failure) {
	level := LevelWarn
	if failure.Kind == htmlupdate.FailureUnknownComponent {
		level = LevelInfo
	}
	Logger(r.Context()).Log(r.Context(), level, "update request refused",
		String("kind", failure.Kind.String()), String("component", failure.KindID),
		String("instance", failure.InstanceID), Err(failure.Err))
	if responseCommitted(w) {
		return
	}
	htmlupdate.WriteFailure(w, failure)
}

// serveUpdate answers a negotiated update request and reports whether it did.
//
// It runs after the live mode has been tested, because both travel on the same
// header and the module resolves anything it does not recognize to a complete
// document: an untested live request would be answered as a page rather than a
// delivery stream.
//
// Every branch here is a mode the client asked for. Absent, unknown, from
// another build, or on a method a delta may not use, the caller keeps the
// document path it was already on, so nothing about an ordinary request changes.
func serveUpdate(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment,
	config HTMLConfig, options []HTMLOption,
) bool {
	update := updateOptions(config)
	if update.Negotiate(r).Mode != htmlupdate.ModeNavigation {
		return false
	}
	async := htmlbind.HasAwaitBlock(wrappers, leaf)
	// The delta is written by the module, which reports no boundary back here,
	// so this render opens the one span it can measure honestly: the whole
	// comparison and every region it decided to send.
	ctx, render := startRenderTrace(requestContext(r), renderModeNavigate,
		chainRenderAttributes(wrappers, async, htmlbind.HasLiveBlock(wrappers, leaf), false)...)
	defer render.end()
	// The module adds the Vary entries itself, so a cache that cannot tell a
	// delta from a document never answers either with the other.
	ctx, cancel := boundedRenderContext(ctx, config, async, false)
	defer cancel()
	// The streaming entry is the one that takes render options, and it is also
	// the one that settles an await boundary rather than dropping it: a delta
	// for a chain with a boundary has to resolve it like the document does.
	// The request is passed unchanged: the module reads it for negotiation and
	// headers, and the render context it hands to template work is the one
	// above, which already carries the span.
	if err := update.RenderStreamAsync(ctx, w, r, wrappers, leaf,
		append(renderOptions(ctx, config, false, nil), options...)...); err != nil {
		render.failed(err)
		// A delta commits with its first record, so a failure after that can
		// only travel in band; the module writes it there and returns it here
		// for the log. Before the first record nothing is committed and the
		// ordinary problem path still applies.
		if responseCommitted(w) {
			Logger(ctx).Log(ctx, LevelError, "update delta failed after commit", Err(err))
			return true
		}
		WriteProblem(w, r, InternalServerError(err))
	}
	return true
}

// reloadableState holds the components this deployment publishes for redraw.
//
// Nothing is registered implicitly. Being exported and single rooted is not
// enough, because registration publishes an HTTP endpoint whose parameters
// anyone can supply: a component that only formats values handed to it is safe,
// while one that loads a record by identifier must check ownership itself.
// Registration is the review point, so it is a deliberate call.
var reloadableState = struct {
	sync.Mutex
	registry *htmlupdate.Registry
	count    int
	failure  error
}{registry: &htmlupdate.Registry{}}

// RegisterReloadable publishes one generated component as a redraw endpoint.
//
// A repeated kind is a startup error rather than a silent overwrite: the kind
// covers a component's name, parameters, and markup but not its package, so two
// identical templates in different packages produce the same one and the wrong
// component could answer.
//
// The failure is also kept, because the ordinary caller is a generated init
// beside the component it registers. An init has nowhere to return an error to,
// and panicking there would end the process before any of the framework's
// logging exists to say which component collided, so startup reports it instead.
// A project registering by hand still gets it here and can decide for itself.
func RegisterReloadable(components ...htmlupdate.Reloadable) error {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	for _, component := range components {
		if err := reloadableState.registry.Register(component); err != nil {
			err = fmt.Errorf("popcornwave: reloadable component: %w", err)
			if reloadableState.failure == nil {
				reloadableState.failure = err
			}
			return err
		}
		reloadableState.count++
	}
	return nil
}

// validateReloadableRegistration reports a registration that failed before main
// ran.
//
// It is checked whatever html.update.enabled says. A collision is not a
// deployment choice but a defect in what generation produced: two components
// asked to publish one endpoint, and whichever registered second is unreachable
// on every deployment that ever turns updates on. Refusing here is the whole
// point of returning an error from a call an init cannot handle.
func validateReloadableRegistration() error {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	return reloadableState.failure
}

// reloadableRegistry is the set the redraw endpoint serves, or nil when a
// project publishes none.
func reloadableRegistry() *htmlupdate.Registry {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	if reloadableState.count == 0 {
		return nil
	}
	return reloadableState.registry
}

// Redraw answers a redraw request for the components named here, and reports
// whether it did. A caller that gets true has had its whole response written.
//
// It belongs at the top of a handler, after that handler's own authorization:
//
//	func orders(w http.ResponseWriter, r *http.Request) {
//		if !mayView(r) { pw.WriteProblem(w, r, pw.Forbidden(nil)); return }
//		if pw.Redraw(w, r, templates.OrderRowReloadable) { return }
//		// ordinary page render
//	}
//
// The address is the page's own URL, which is the whole reason this is a call
// rather than an endpoint. Path protection is configured by path pattern, so a
// redraw on a reserved path needs a second pattern kept in step with the one
// protecting the page the component sits on — two rules that must agree with
// nothing forcing them to. Here the redraw is the same request as the page, so
// it inherits the page's protection, and placed after the handler's own checks
// it inherits those too rather than only the middleware's.
//
// Naming the components is what bounds the surface: this handler answers for
// these and nothing else, so a page cannot be asked to render a component it
// never shows. That set is readable in Go beside the check that guards it.
//
// It stays the caller's job to authorize the arguments. Every parameter but the
// instance id arrives from whoever issued the request, so a component that loads
// a record by identifier verifies ownership itself exactly as a handler does.
// ReloadablePage is implemented by a generated component's parameter struct when
// that component's markup can contain a reloadable one.
//
// Nobody writes it. Generation folds each component's call graph and emits the
// method on the components it reaches something from, which is what lets Redraw
// take the page itself rather than a list the author has to keep in step with
// the template.
type ReloadablePage interface {
	PwReloadables() []htmlupdate.Reloadable
}

// Redraw answers a redraw for what this page's markup can contain, and reports
// whether it did. A caller that gets true has had its whole response written.
//
// It takes the page component, so the set comes from the template rather than
// from a list beside it:
//
//	func orders(w http.ResponseWriter, r *http.Request) {
//		if !mayView(r) { pw.WriteProblem(w, r, pw.Forbidden(nil)); return }
//		if pw.Redraw(w, r, templates.OrdersPage) { return }
//		// the query the page needs, which a redraw has now skipped
//	}
//
// The component is named, not called, so nothing builds its parameters and the
// data behind them is never fetched. That is the point of answering here rather
// than inside the page render, where the same redraw costs the whole page's
// preparation.
//
// A page whose markup reaches no reloadable component does not satisfy
// ReloadablePage and will not compile here. That is the honest answer: there is
// nothing on that page to redraw.
//
// It belongs after the handler's own authorization. The address is the page's
// own URL, so the redraw already inherits what guards the page; placing the call
// below the checks is what extends that to the handler's own.
func Redraw[P ReloadablePage](w http.ResponseWriter, r *http.Request, page func(P) HTMLFragment) bool {
	_ = page
	var declared P
	return RedrawComponents(w, r, declared.PwReloadables()...)
}

// RedrawComponents answers a redraw for the components named here.
//
// It is the escape hatch behind Redraw, for a handler that renders something
// other than a generated page component, or that publishes a narrower set than
// its template reaches. Prefer Redraw, which cannot fall out of step with the
// markup.
func RedrawComponents(w http.ResponseWriter, r *http.Request, components ...htmlupdate.Reloadable) bool {
	if len(components) == 0 {
		return false
	}
	config := Config[HTMLConfig](requestContext(r))
	if !config.Update.Enabled {
		return false
	}
	options := updateOptions(config)
	// The mode is tested before the registry is built, because this call sits on
	// the ordinary page path and runs on every request to it. Registration
	// encodes each component's head to check its bound, which is work no
	// document request should pay for.
	if options.Negotiate(r).Mode != htmlupdate.ModeRedraw {
		return false
	}
	registry := &htmlupdate.Registry{}
	for _, component := range components {
		if err := registry.Register(component); err != nil {
			// A duplicate kind or an oversized head is a defect in what this
			// handler named rather than anything the request did, so it is
			// reported through the failure path like any other refusal.
			writeUpdateFailure(w, r, htmlupdate.Failure{
				Kind:    htmlupdate.FailureRenderFailed,
				Status:  http.StatusInternalServerError,
				Message: "redraw registry",
				Err:     err,
			})
			return true
		}
	}
	// The mode is already known here, so the span opens without negotiating a
	// second time.
	_, render := startRenderTrace(requestContext(r), renderModeRedraw, Int("pw.render.layers", 1))
	defer render.end()
	return options.Redraw(w, render.request(r), registry)
}

// serveRegisteredRedraw answers a redraw from the process-wide published set.
//
// It is the page tree's half of the same capability. A generated route handler
// runs its Load and then calls the render entry, so the branch lands there: the
// page's authorization has already run and already returned its own error, and
// the redraw is refused exactly when the page would be. The cost is the one data
// fetch the redraw did not need, which is the price of not having a seam of its
// own inside a generated handler.
//
// Calling Redraw first is the faster path and the narrower one. It is answered
// before whatever the handler does to build its page, so a redraw pays for no
// data the page needed and none it did not, and the set it names bounds what the
// URL can be asked to render. This is the same capability either way; the
// difference is what the redraw costs and how much of the published set one URL
// exposes.
func serveRegisteredRedraw(w http.ResponseWriter, r *http.Request, config HTMLConfig) bool {
	registry := reloadableRegistry()
	if registry == nil {
		return false
	}
	options := updateOptions(config)
	// Redraw declines before it renders anything, and this call sits on the
	// ordinary page path, so the mode is tested here rather than leaving a span
	// open around a call that turned out to be a document request. The second
	// negotiation is paid only while tracing is on.
	if !renderTraced(requestContext(r)) || options.Negotiate(r).Mode != htmlupdate.ModeRedraw {
		return options.Redraw(w, r, registry)
	}
	_, render := startRenderTrace(requestContext(r), renderModeRedraw, Int("pw.render.layers", 1))
	defer render.end()
	return options.Redraw(w, render.request(r), registry)
}

// WantsUpdate reports whether the caller can apply an update response.
//
// It is the one branch point of an action handler. An ordinary form submission
// and a non-browser client cannot apply one, so they take the response the
// handler already wrote — a redirect, or JSON — and a page with the runtime
// takes the regions instead. Keeping it to one predicate is what stops the two
// paths from drifting apart.
func WantsUpdate(r *http.Request) bool {
	config := Config[HTMLConfig](requestContext(r))
	if !config.Update.Enabled {
		return false
	}
	return updateOptions(config).WantsUpdate(r)
}

// UpdateRegion pairs a target element id with the fragment replacing it.
type UpdateRegion = htmlupdate.Update

// Replace names one region an action response rewrites. The rendered root
// element must carry the same id, or the region becomes unaddressable after the
// first update.
func Replace(targetID string, fragment HTMLFragment) UpdateRegion {
	return htmlupdate.Replace(targetID, fragment)
}

// WriteUpdate answers a mutating request with the regions its action changed,
// so one round trip both performs the action and refreshes the page.
//
// The status is the handler's own. The browser applies the regions whatever it
// says, because a rejected submission returns 4xx and the regions it carries
// are the validation errors — showing them is the point. That is the opposite
// of a redraw, where a non-2xx means the render failed.
func WriteUpdate(w http.ResponseWriter, r *http.Request, status int, regions ...UpdateRegion) {
	config := Config[HTMLConfig](requestContext(r))
	if err := updateOptions(config).WriteUpdateStatus(w, status, regions...); err != nil {
		// Nothing is written until every region rendered, so a failure here can
		// still choose its own status.
		WriteProblem(w, r, InternalServerError(err))
	}
}

// WriteUpdateNavigate tells the browser to leave the page, which is how an
// action that changed where the user belongs stays correct without guessing
// which regions to rewrite.
//
// The target is refused unless it is one a browser can follow without running
// script. The value reaching here is commonly a return path taken from the
// request — the shape of a post-login redirect — and the browser runtime hands
// it to location.assign, which executes a javascript: URL rather than
// navigating to it. Refusing here means an application cannot turn its own
// redirect into script execution by forwarding a parameter it did not check.
func WriteUpdateNavigate(w http.ResponseWriter, r *http.Request, url string) {
	if !safeurl.Navigable(url) {
		// The URL itself stays out of the error: it is request-derived, and a
		// 5xx body is sanitized anyway, so repeating it would only risk placing
		// it somewhere that is not.
		WriteProblem(w, r, InternalServerError(errUnsafeNavigation))
		return
	}
	config := Config[HTMLConfig](requestContext(r))
	if err := updateOptions(config).WriteNavigate(w, url); err != nil {
		WriteProblem(w, r, InternalServerError(err))
	}
}

// errUnsafeNavigation reports a navigation target this framework will not hand
// to a browser. It is a programming error rather than a request error: the
// handler chose the target, so the fix is in the handler.
var errUnsafeNavigation = errors.New("popcornwave: navigation target is not a URL a browser can follow without running script")
