package pwfast

import (
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
	for name, values := range response.Header {
		for _, value := range values {
			r.Response.Header.Add(name, value)
		}
	}
	if response.Status != 0 {
		r.SetStatusCode(response.Status)
	}
	_, _ = r.Write(response.Body)
}

// errUnsafeNavigation reports a navigation target this framework will not hand
// to a browser. It is a programming error rather than a request error: the
// handler chose the target, so the fix is in the handler.
var errUnsafeNavigation = errors.New("popcornwave: navigation target is not a URL a browser can follow without running script")

// errUpdatesDisabled reports an update entry called by a project that never
// enabled the feature, which is a wiring mistake rather than a request one.
var errUpdatesDisabled = errors.New("popcornwave: partial updates are not enabled; set html.update.enabled")
