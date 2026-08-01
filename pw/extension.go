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

// Slot orders framework extensions in the request chain. A smaller slot runs
// earlier, so a guard always observes the session and authentication state
// established before it.
type Slot int

const (
	// SlotStorage installs storage clients that later slots resolve from the
	// request context. A session backend reads its store here, so this runs
	// before SlotSession.
	SlotStorage Slot = 5
	// SlotSession resolves stored session state.
	SlotSession Slot = 10
	// SlotAuthentication finalizes the request authentication result and owns
	// its own login, callback, and logout paths.
	SlotAuthentication Slot = 20
	// SlotGuard rejects unauthenticated requests to protected paths.
	SlotGuard Slot = 30
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

// applyExtensions wraps handler with every registered extension. The chain is
// built inward-out so that the lowest slot ends up outermost.
func applyExtensions(ctx context.Context, handler http.Handler) (http.Handler, error) {
	extensionState.Lock()
	ordered := append([]Extension(nil), extensionState.registered...)
	extensionState.Unlock()
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Slot < ordered[j].Slot })

	// Setup runs in slot order so an extension may depend on state prepared by
	// an earlier slot. The chain is composed afterwards, in reverse, so the
	// lowest slot ends up outermost.
	middlewares := make([]Middleware, len(ordered))
	for index, extension := range ordered {
		middleware, err := extension.Setup(ctx)
		if err != nil {
			return nil, fmt.Errorf("popcornwave: %s: %w", extension.Name, err)
		}
		middlewares[index] = middleware
		if extension.Close != nil {
			registerCleanup(extension.Name, extension.Close)
		}
	}
	result := handler
	for index := len(middlewares) - 1; index >= 0; index-- {
		if middlewares[index] != nil {
			result = middlewares[index](result)
		}
	}
	return result, nil
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
