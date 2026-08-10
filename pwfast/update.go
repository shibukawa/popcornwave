package pwfast

import (
	"context"
	"errors"
	"net/http"

	"github.com/shibukawa/popcornwave/internal/safeurl"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/fasthttpupdate"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// UpdateRegion pairs a target element id with the fragment replacing it.
type UpdateRegion = pwruntime.UpdateRegion

// Replace names one region an action response rewrites. The rendered root
// element must carry the same id, or the region becomes unaddressable after the
// first update.
func Replace(targetID string, fragment HTMLFragment) UpdateRegion {
	return fasthttpupdate.Replace(targetID, fragment)
}

// RegisterReloadable publishes generated components as redraw endpoints,
// reaching the one registry decision:shared-runtime-leaf keeps.
func RegisterReloadable(components ...pwruntime.UpdateReloadable) error {
	return pwruntime.RegisterReloadable(components...)
}

// updateOptions builds this runtime's options from the resolved configuration.
//
// The configuration is read once by whichever runtime resolved it and published
// transport-free, because a settings file is not a transport concern and this
// half has no reader of its own. Absent settings mean nothing enabled updates,
// which every entry below treats as "not an update request" rather than as an
// error: a project that never turned the feature on should see the ordinary
// response, not a failure about a feature it does not use.
func updateOptions() (fasthttpupdate.Options, bool) {
	settings, ok := pwruntime.ResolvedUpdateSettings()
	if !ok || !settings.Enabled {
		return fasthttpupdate.Options{}, false
	}
	return fasthttpupdate.Options{
		Key:                 []byte(settings.ValidatorKey),
		HeaderPrefix:        settings.HeaderPrefix,
		DataAttributePrefix: settings.DataAttributePrefix,
		GlobalName:          settings.GlobalName,
		PathPrefix:          settings.PathPrefix,
		BuildID:             settings.BuildID,
		MaxManifestBytes:    settings.MaxManifestBytes,
		CSRFHeaderName:      settings.CSRFHeaderName,
		CallerOwnsRuntime:   settings.CallerOwnsRuntime,
		OnFailure:           pwruntime.LogUpdateRefusal,
	}, true
}

// WantsUpdate reports whether the caller can apply an update response.
//
// It is the one branch point of an action handler. An ordinary form submission
// and a non-browser client cannot apply one, so they take the response the
// handler already wrote, and a page carrying the runtime takes the regions
// instead. Keeping it to one predicate is what stops the two paths from
// drifting apart.
func WantsUpdate(r *fasthttp.RequestCtx) bool {
	options, ok := updateOptions()
	if !ok {
		return false
	}
	return options.WantsUpdate(r)
}

// WriteUpdate answers a mutating request with the regions its action changed,
// so one round trip both performs the action and refreshes the page.
//
// The status is the handler's own. The browser applies the regions whatever it
// says, because a rejected submission returns 4xx and the regions it carries
// are the validation errors — showing them is the point. That is the opposite
// of a redraw, where a non-2xx means the render failed.
func WriteUpdate(r *fasthttp.RequestCtx, status int, regions ...UpdateRegion) {
	options, ok := updateOptions()
	if !ok {
		WriteProblem(r, InternalServerError(errUpdatesDisabled))
		return
	}
	// Nothing is written until every region rendered, so a failure here can
	// still choose its own status.
	response, err := options.WriteUpdateStatus(r, status, regions)
	if err != nil {
		WriteProblem(r, InternalServerError(err))
		return
	}
	writeUpdateResponse(r, response)
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
func WriteUpdateNavigate(r *fasthttp.RequestCtx, url string) {
	if !safeurl.Navigable(url) {
		// The URL itself stays out of the error: it is request-derived, and a
		// 5xx body is sanitized anyway, so repeating it would only risk placing
		// it somewhere that is not.
		WriteProblem(r, InternalServerError(errUnsafeNavigation))
		return
	}
	options, ok := updateOptions()
	if !ok {
		WriteProblem(r, InternalServerError(errUpdatesDisabled))
		return
	}
	response, err := options.WriteNavigate(url)
	if err != nil {
		WriteProblem(r, InternalServerError(err))
		return
	}
	writeUpdateResponse(r, response)
}

// RedrawComponents answers a redraw request for the components named here, and
// reports whether it did. A caller that gets true has had its whole response
// written.
//
// It belongs at the top of a handler, after that handler's own authorization: a
// redraw reaches the component through the same route, so whatever guards the
// page guards the region.
func RedrawComponents(r *fasthttp.RequestCtx, components ...pwruntime.UpdateReloadable) bool {
	options, ok := updateOptions()
	if !ok {
		return false
	}
	registry := &pwruntime.UpdateRegistry{}
	for _, component := range components {
		if err := registry.Register(component); err != nil {
			// A duplicate kind or an oversized head is a defect in what this
			// handler named rather than anything the request did, so it is
			// reported through the failure path like any other refusal.
			failure := pwruntime.UpdateFailure{
				Kind:    fasthttpupdate.FailureRenderFailed,
				Status:  http.StatusInternalServerError,
				Message: "redraw registry",
				Err:     err,
			}
			pwruntime.LogUpdateRefusal(r, failure)
			writeUpdateResponse(r, fasthttpupdate.FailureResponse(failure))
			return true
		}
	}
	return answerRedraw(r, options, registry)
}

// Redraw answers a redraw from the process-wide published set, which is what a
// generated page route calls after its own Load has run.
func Redraw(r *fasthttp.RequestCtx) bool {
	options, ok := updateOptions()
	if !ok {
		return false
	}
	registry := pwruntime.ReloadableRegistry()
	if registry == nil {
		return false
	}
	return answerRedraw(r, options, registry)
}

// answerRedraw sends what the module composed, when it composed anything.
//
// The bool is unchanged in meaning: false says the request was not a redraw at
// all, and the caller falls through to the page. A refusal is an answer rather
// than a false, and it has already reached the failure hook by the time it
// arrives here.
func answerRedraw(r *fasthttp.RequestCtx, options fasthttpupdate.Options, registry *pwruntime.UpdateRegistry) bool {
	response, answered := options.Redraw(r, registry)
	if !answered {
		return false
	}
	writeUpdateResponse(r, response)
	return true
}

// writeUpdateResponse sends a composed answer.
//
// The module builds a response and leaves the sending to its caller, which is
// what lets this half exist: the value carries a status, a header set, and a
// body, and none of the three names a transport. Only this function does.
func writeUpdateResponse(r *fasthttp.RequestCtx, response fasthttpupdate.Response) {
	applyHeader(r, response.Header)
	if response.Status != 0 {
		r.SetStatusCode(response.Status)
	}
	_, _ = r.Write(response.Body)
}

// applyHeader copies a composed header set onto the response.
//
// The header type is net/http's on both sides because the module composes it,
// and it is an ordinary map rather than a transport: nothing here reads a
// request or writes a body. Only the destination differs, which is the whole
// reason a second copier exists.
func applyHeader(r *fasthttp.RequestCtx, header http.Header) {
	for name, values := range header {
		for _, value := range values {
			r.Response.Header.Add(name, value)
		}
	}
}

// errUnsafeNavigation reports a navigation target this framework will not hand
// to a browser. It is a programming error rather than a request error: the
// handler chose the target, so the fix is in the handler.
var errUnsafeNavigation = errors.New("popcornwave: navigation target is not a URL a browser can follow without running script")

// errUpdatesDisabled reports an update entry called by a project that never
// enabled the feature, which is a wiring mistake rather than a request one.
var errUpdatesDisabled = errors.New("popcornwave: partial updates are not enabled; set html.update.enabled")

// ServeUpdate answers a negotiated streamed navigation with the delta of one
// chain, and reports whether it did.
//
// The module owns the comparison and the framing; what this supplies is the
// headers a stream must carry before its first record, and the request value it
// writes into. Both come from the same entry pw calls, so the two transports
// send the same records rather than two implementations that agree.
func ServeUpdate(r *fasthttp.RequestCtx, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) bool {
	settings, ok := pwruntime.ResolvedUpdateSettings()
	if !ok || !settings.Enabled {
		return false
	}
	update, ok := updateOptions()
	if !ok {
		return false
	}
	if update.Negotiate(r).Mode != fasthttpupdate.ModeNavigation {
		return false
	}
	// A stream commits with its first record, so everything the response has to
	// carry goes on before the render starts: the axes that keep a cache from
	// answering a document request with a delta, the framing, and the mode
	// echoed back.
	applyHeader(r, update.StreamHeaders(r, wrappers, leaf))
	r.Response.Header.Set("Cache-Control", updateCacheControl)
	ctx, cancel := boundedRenderContext(r, settings)
	defer cancel()
	render := append(settings.RenderOptions(ctx), options...)
	if err := update.RenderStreamAsync(ctx, r, wrappers, leaf, render...); err != nil {
		// A delta commits with its first record, so a failure after that can
		// only travel in band; the module writes it there and returns it here
		// for the log. Before the first record nothing is committed and the
		// ordinary problem path still applies.
		if r.Response.Header.StatusCode() != 0 && len(r.Response.Body()) > 0 {
			pwruntime.ReadLogger(ctx).Log(ctx, pwruntime.LevelError,
				"update stream failed after commit", pwruntime.Err(err))
			return true
		}
		WriteProblem(r, InternalServerError(err))
	}
	return true
}

// Live delivery is deliberately absent, and this is where it would go.
//
// A first cut called the module's own live entry, which compiles and is wrong:
// the net/http half does not use that entry. It runs its own loop over the
// chain renderer and layers on what makes live usable — admission control per
// client, a lifetime and idle watchdog, digest suppression seeded from the
// client's manifest so a reconnect re-sends only what changed, a bound on
// boundaries, and the render telemetry. None of that exists in the module
// entry, so this half would have answered the same requests with a poorer
// stream and no way for anyone to notice.
//
// The convergence runs the other way: the transport-free majority of that loop
// — the digests, the manifest, the watchdog, the admission, the records, the
// close reasons — belongs in the shared leaf, leaving each runtime the headers,
// the write, and the flush. Both halves then run the richer implementation
// rather than one running a thinner one.
//
// boundedRenderContext applies the configured boundary bound to a render.
//
// A streamed answer settles its await boundaries as it goes, and without this a
// chain whose source never answers would hold the response open until the
// request context ended, which is the stall the timeout exists to prevent.
func boundedRenderContext(r *fasthttp.RequestCtx, settings pwruntime.UpdateSettings) (context.Context, context.CancelFunc) {
	if settings.AsyncTimeout <= 0 {
		return r, func() {}
	}
	return context.WithTimeout(r, settings.AsyncTimeout)
}

// updateCacheControl keeps a delta and a live delivery out of every cache. A
// shared cache holding one would answer another reader's page with it.
const updateCacheControl = "private, no-store"
