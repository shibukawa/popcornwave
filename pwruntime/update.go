package pwruntime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// The partial-update types are the module's shared ones. htmlupdate and its
// fasthttp sibling both alias a single declaration of each, so naming them
// through either package names one type — and one registry therefore serves
// both runtimes, which is what this file exists to make true.
//
// They are reached through htmlupdate rather than the fasthttp package because
// this leaf already imports net/http for status constants, where importing the
// other would put the fasthttp fork into every project that never opens a
// socket with it.
type (
	UpdateRegistry   = htmlupdate.Registry
	UpdateReloadable = htmlupdate.Reloadable
	UpdateFailure    = htmlupdate.Failure
	UpdateRegion     = htmlupdate.Update
)

// UpdateSettings is the resolved partial-update configuration in a form that
// names no transport, so one resolution serves both runtimes.
//
// It is values rather than a composed options struct because the two runtimes
// each declare their own, with the same fields and different method receivers.
// Copying the fields twice is cheaper than a conversion whose correctness would
// depend on two struct declarations staying identical.
type UpdateSettings struct {
	// Enabled is the update surface: navigation deltas, redraws, and action
	// responses. It gates those and nothing else.
	Enabled bool
	// Live is the subscription that keeps a page updating after its document is
	// complete. It is a separate switch because it answers a separate request,
	// and gating it on the one above would turn a project that asked only for
	// live rendering into one that got none — which is how the second transport
	// answered a subscription with a whole document.
	Live                bool
	ValidatorKey        string
	HeaderPrefix        string
	DataAttributePrefix string
	GlobalName          string
	PathPrefix          string
	BuildID             string
	MaxManifestBytes    int
	CSRFHeaderName      string
	CallerOwnsRuntime   bool
	// AsyncTimeout bounds one await boundary and AsyncConcurrency bounds the
	// boundary work running at once. They travel with the update settings
	// because a streamed answer renders the same chain a document does, and a
	// runtime that could not read them would settle boundaries on terms the
	// deployment did not choose.
	AsyncTimeout     time.Duration
	AsyncConcurrency int
	// The live bounds, which the delivery loop reads on either transport. They
	// travel here for the reason the async bounds do: the loop is shared, so
	// the values it consults have to be.
	LiveMaxResponses   int
	LiveMaxBoundaries  int
	LiveMaxDuration    time.Duration
	LiveDurationJitter int
	LiveIdleTimeout    time.Duration
}

// RenderOptions is the option set a streamed answer renders with, built from
// the published settings so both runtimes bound a boundary the same way.
//
// It is the shared subset rather than everything pw assembles: the cache store
// and its scope are resolved from the request context by whichever runtime owns
// that resolution, and a caller adds them after these.
func (s UpdateSettings) RenderOptions(ctx context.Context) []htmlbind.Option {
	options := []htmlbind.Option{
		htmlbind.WithContext(ctx),
		htmlbind.WithErrorReporter(func(err error) {
			ReadLogger(ctx).Log(ctx, LevelError, "await boundary failed", Err(err))
		}),
	}
	if s.AsyncTimeout > 0 {
		options = append(options, htmlbind.WithAsyncTimeout(s.AsyncTimeout))
	}
	if s.AsyncConcurrency > 0 {
		options = append(options, htmlbind.WithConcurrencyLimit(s.AsyncConcurrency))
	}
	return options
}

var updateSettingsState atomic.Pointer[UpdateSettings]

// PublishUpdateSettings records the resolved configuration for whichever
// runtime reads it. The transport that resolved the configuration and the
// transport that serves the request need not be the same one.
func PublishUpdateSettings(settings UpdateSettings) {
	updateSettingsState.Store(&settings)
}

// ResolvedUpdateSettings returns the published configuration, and whether
// anything published one. A runtime that finds none has nothing to answer an
// update request with and says so rather than guessing at defaults.
func ResolvedUpdateSettings() (UpdateSettings, bool) {
	settings := updateSettingsState.Load()
	if settings == nil {
		return UpdateSettings{}, false
	}
	return *settings, true
}

// reloadableState holds the components this deployment publishes for redraw.
//
// Nothing is registered implicitly. Being exported and single rooted is not
// enough, because registration publishes an HTTP endpoint whose parameters
// anyone can supply: a component that only formats values handed to it is safe,
// while one that loads a record by identifier must check ownership itself.
// Registration is the review point, so it is a deliberate call.
//
// It lives here for the reason the document shell does: generated registration
// reaches whichever runtime it imports, and two registries would leave one
// build answering no redraw at all.
var reloadableState = struct {
	sync.Mutex
	registry *UpdateRegistry
	count    int
	failure  error
}{registry: &UpdateRegistry{}}

// RegisterReloadable publishes generated components as redraw endpoints.
//
// A repeated kind is an error rather than a silent overwrite: the kind covers a
// component's name, parameters, and markup but not its package, so two
// identical templates in different packages produce the same one and the wrong
// component could answer.
//
// The failure is also kept, because the ordinary caller is a generated init
// beside the component it registers, and an init has nowhere to return to.
func RegisterReloadable(components ...UpdateReloadable) error {
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

// ReloadableRegistry returns the published set, or nil where nothing published
// one, which is what tells a caller there is no redraw endpoint to serve.
func ReloadableRegistry() *UpdateRegistry {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	if reloadableState.count == 0 {
		return nil
	}
	return reloadableState.registry
}

// ReloadableRegistrationFailure reports a registration that failed before main
// ran, whatever the configuration says: a collision is a defect in what
// generation produced rather than a deployment choice.
func ReloadableRegistrationFailure() error {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	return reloadableState.failure
}

// ResetReloadableForTest restores the registry to a known state and returns
// what it replaced, so a test that registers can put back what it found.
func ResetReloadableForTest() (*UpdateRegistry, int, error) {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	registry, count, failure := reloadableState.registry, reloadableState.count, reloadableState.failure
	reloadableState.registry, reloadableState.count, reloadableState.failure = &UpdateRegistry{}, 0, nil
	return registry, count, failure
}

// RestoreReloadableForTest puts back what ResetReloadableForTest returned.
func RestoreReloadableForTest(registry *UpdateRegistry, count int, failure error) {
	reloadableState.Lock()
	defer reloadableState.Unlock()
	reloadableState.registry, reloadableState.count, reloadableState.failure = registry, count, failure
}

// LogUpdateRefusal records a refused update request.
//
// Both runtimes install it as their failure hook, so one refusal reads the same
// whichever transport served it. Version skew is the ordinary case — a page
// loaded before a deploy asks for a component whose markup has changed, gets a
// 404, and reloads — so it is recorded rather than treated as a fault.
func LogUpdateRefusal(ctx context.Context, failure UpdateFailure) {
	level := LevelWarn
	if failure.Kind == htmlupdate.FailureUnknownComponent {
		level = LevelInfo
	}
	ReadLogger(ctx).Log(ctx, level, "update request refused",
		String("kind", failure.Kind.String()), String("component", failure.KindID),
		String("instance", failure.InstanceID), Err(failure.Err))
}
