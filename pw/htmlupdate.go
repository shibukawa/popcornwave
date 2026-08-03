package pw

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"

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
func writeUpdateFailure(w http.ResponseWriter, r *http.Request, failure htmlupdate.Failure) {
	level := LevelWarn
	if failure.Kind == htmlupdate.FailureUnknownComponent || failure.Kind == htmlupdate.FailureStalePage {
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
	// The module adds the Vary entries itself, so a cache that cannot tell a
	// delta from a document never answers either with the other.
	ctx, cancel := boundedRenderContext(requestContext(r), config, htmlbind.HasAwaitBlock(wrappers, leaf), false)
	defer cancel()
	// The streaming entry is the one that takes render options, and it is also
	// the one that settles an await boundary rather than dropping it: a delta
	// for a chain with a boundary has to resolve it like the document does.
	if err := update.RenderStreamAsync(ctx, w, r, wrappers, leaf,
		append(renderOptions(ctx, config, false, nil), options...)...); err != nil {
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
}{registry: &htmlupdate.Registry{}}

// RegisterReloadable publishes one generated component as a redraw endpoint.
//
// It is intended for generated page registry code. A repeated kind is a startup
// error rather than a silent overwrite: the kind covers a component's name,
// parameters, and markup but not its package, so two identical templates in
// different packages produce the same one and the wrong component could answer.
func RegisterReloadable(components ...htmlupdate.Reloadable) error {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	for _, component := range components {
		if err := reloadableState.registry.Register(component); err != nil {
			return fmt.Errorf("popcornwave: reloadable component: %w", err)
		}
		reloadableState.count++
	}
	return nil
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

// serveRedraw answers a registered component's redraw request and reports
// whether it handled one.
//
// It sits inside the reserved prefix beside the runtime asset, so one routing,
// caching, and access rule covers everything this framework owns.
func serveRedraw(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, updatePathPrefix+"/redraw/") {
		return false
	}
	config := Config[HTMLConfig](requestContext(r))
	registry := reloadableRegistry()
	if !config.Update.Enabled || registry == nil {
		// A project that publishes nothing answers 404 here rather than
		// falling through to application routing, which is the reserved
		// prefix's rule for every path it does not serve.
		http.NotFound(w, r)
		return true
	}
	updateOptions(config).RedrawHandler(registry).ServeHTTP(w, r)
	return true
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
func WriteUpdateNavigate(w http.ResponseWriter, r *http.Request, url string) {
	config := Config[HTMLConfig](requestContext(r))
	if err := updateOptions(config).WriteNavigate(w, url); err != nil {
		WriteProblem(w, r, InternalServerError(err))
	}
}
