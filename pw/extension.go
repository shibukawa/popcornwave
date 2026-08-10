package pw

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Middleware is the standard net/http middleware shape used by framework
// extensions.
type Middleware = func(http.Handler) http.Handler

// Slot orders every frame of the request chain. A smaller slot runs earlier,
// which is to say outermost, so a guard always observes the session and
// authentication state established before it.
//
// Framework frames sit at multiples of ten, BASIC style, so a middleware can
// be inserted between any two by picking a number in the gap:
// pw.SlotAccessLog - 5 runs after the request ID is minted and before the
// access log times the request.
type Slot int

const (
	// SlotTracing opens the request root span. The frame is installed only
	// when tracing has somewhere to export.
	SlotTracing Slot = 10
	// SlotResources injects the logger, configuration, and database clients
	// into the request context. Every frame below reads them from there.
	SlotResources Slot = 20
	// SlotClientAddress resolves the caller's own address against the declared
	// proxy networks, so everything below counts one client rather than one
	// relay. It sits above the access log because a record naming the proxy
	// for every request is a record that names nothing.
	SlotClientAddress Slot = 25
	// SlotRequestID validates or mints the ID every log line carries.
	SlotRequestID Slot = 30
	// SlotAccessLog writes one structured line per request, with timing.
	SlotAccessLog Slot = 40
	// SlotRecover converts a panic below it into a negotiated error response.
	SlotRecover Slot = 50
	// SlotRateLimitProcess refuses arrivals above the total ceiling.
	//
	// It sits below SlotAccessLog and SlotRecover, so a refusal is still one
	// logged line rather than a silent drop, and above everything that costs
	// storage or a session, because a valve is worth less the further in it
	// sits. It is also the only layer that sees a flood spread across many
	// addresses, each staying under its own bucket by construction.
	SlotRateLimitProcess Slot = 55
	// SlotSecurityHeaders sets policy headers before anything writes.
	SlotSecurityHeaders Slot = 60
	// SlotRequestTimeout bounds the whole request.
	SlotRequestTimeout Slot = 70
	// SlotMaxRequestBody caps downstream reads of the request body.
	SlotMaxRequestBody Slot = 80
	// SlotPublicAssets serves the static tree before any dynamic work.
	SlotPublicAssets Slot = 90
	// SlotOperational answers the health and readiness probes and the
	// framework assets, above everything that authenticates. It is a fixed
	// frame: registering at this exact number is refused, because the frame is
	// a handler rather than a middleware and nothing can share its position.
	SlotOperational Slot = 100
	// SlotStorage installs storage clients that later slots resolve from the
	// request context. A session backend reads its store here, so this runs
	// before SlotSession.
	SlotStorage Slot = 110
	// SlotSession resolves stored session state.
	SlotSession Slot = 120
	// SlotAuthentication finalizes the request authentication result and owns
	// its own login, callback, and logout paths.
	SlotAuthentication Slot = 130
	// SlotRateLimit refuses a caller over its own allowance.
	//
	// It sits below SlotAuthentication because one bucket keyed on
	// subject-or-address cannot know which of the two it is until the subject
	// is resolved, and above SlotCSRF so a flood does not pay for token
	// verification on its way to being refused.
	SlotRateLimit Slot = 135
	// SlotCSRF rejects forged unsafe requests.
	//
	// It sits below SlotSession because the token it compares against comes
	// from the resolved session, and below SlotAuthentication so a plugin's own
	// endpoints — a login post, an OIDC callback — answer above it rather than
	// needing a configured exclusion to reach their handlers.
	SlotCSRF Slot = 140
	// SlotGuard rejects unauthenticated requests to protected paths.
	SlotGuard Slot = 150
	// SlotAPIDoc answers the OpenAPI document and its UI beneath the guard, so
	// a map of the API surface costs what the routes it describes cost. It is
	// a fixed frame exactly as SlotOperational is.
	SlotAPIDoc Slot = 160
)

// Extension is one imported framework capability. Setup runs once during
// framework initialization, after configuration parsing and database startup,
// and returns the middleware to install. Returning a nil middleware installs
// nothing, which is how a disabled extension opts out.
type Extension struct {
	Name  string
	Slot  Slot
	Setup func(context.Context) (Middleware, error)
	// Close releases resources owned by the extension during shutdown.
	Close func(context.Context) error
}

var extensionState = struct {
	sync.Mutex
	registered []Extension
}{}

// RegisterExtension adds one extension to the framework chain. Imported
// packages call it from an init function so that only linked capabilities
// contribute configuration and code.
func RegisterExtension(extension Extension) {
	if strings.TrimSpace(extension.Name) == "" {
		panic("popcornwave: empty extension name")
	}
	if extension.Setup == nil {
		panic("popcornwave: extension " + extension.Name + " has no setup")
	}
	extensionState.Lock()
	defer extensionState.Unlock()
	for _, existing := range extensionState.registered {
		if existing.Name == extension.Name {
			panic("popcornwave: duplicate extension " + extension.Name)
		}
	}
	extensionState.registered = append(extensionState.registered, extension)
}

// RegisterMiddleware adds one application middleware to the request chain at
// slot. A smaller slot runs earlier, so pw.SlotAccessLog - 5 observes the
// request ID and appears in the access log's timing, and pw.SlotGuard + 1 runs
// only for requests the guard admitted.
//
// Call it from main, before Run or Middlewares builds the chain, exactly as
// RegisterSessionStore requires: the chain is composed once, and a middleware
// registered after that joins nothing.
//
// The two fixed frames, SlotOperational and SlotAPIDoc, are handlers rather
// than middleware and refuse registration at their exact number; register one
// position to either side instead. A duplicate name and a nil middleware are
// each a panic at registration rather than a silent gap in the chain.
func RegisterMiddleware(slot Slot, name string, middleware Middleware) {
	if middleware == nil {
		panic("popcornwave: middleware " + name + " is nil")
	}
	if slot == SlotOperational || slot == SlotAPIDoc {
		panic(fmt.Sprintf(
			"popcornwave: middleware %s registered at fixed frame %d; pick a neighboring slot relative to pw.SlotOperational or pw.SlotAPIDoc",
			name, slot))
	}
	captured := middleware
	RegisterExtension(Extension{
		Name: name,
		Slot: slot,
		Setup: func(context.Context) (Middleware, error) {
			return captured, nil
		},
	})
}

// chainFrame is one positioned frame of the request chain: a framework
// middleware, a registered application middleware, or one of the fixed
// handler frames adapted to the middleware shape.
type chainFrame struct {
	slot       Slot
	name       string
	middleware Middleware
}

// extensionFrames runs every registered Setup and returns the resulting
// frames. Setup runs in ascending slot order so an extension may depend on
// state prepared by an earlier slot; composing the chain is the caller's job.
func extensionFrames(ctx context.Context) ([]chainFrame, error) {
	extensionState.Lock()
	ordered := append([]Extension(nil), extensionState.registered...)
	extensionState.Unlock()
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Slot < ordered[j].Slot })

	frames := make([]chainFrame, 0, len(ordered))
	for _, extension := range ordered {
		middleware, err := extension.Setup(ctx)
		if err != nil {
			return nil, fmt.Errorf("popcornwave: %s: %w", extension.Name, err)
		}
		if extension.Close != nil {
			registerCleanup(extension.Name, extension.Close)
		}
		// A nil middleware installs nothing, which is how a disabled
		// capability opts out.
		if middleware == nil {
			continue
		}
		frames = append(frames, chainFrame{slot: extension.Slot, name: extension.Name, middleware: middleware})
	}
	return frames, nil
}

// registerCleanup adds a shutdown hook that Run executes in reverse order.
// Repeated framework initialization, which tests perform, keeps one hook per
// extension name.
func registerCleanup(name string, fn func(context.Context) error) {
	configState.Lock()
	defer configState.Unlock()
	for _, existing := range configState.cleanups {
		if existing.name == name {
			return
		}
	}
	configState.cleanups = append(configState.cleanups, &runtimeCleanup{name: name, fn: fn})
}
