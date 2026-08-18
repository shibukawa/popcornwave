package pw

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/shibukawa/popcornweb/middlewares"
	"github.com/shibukawa/popcornweb/pwruntime"
)

func init() {
	// Two frames, one store. The ceiling belongs in the outer stack where a
	// refusal has cost least, and the identity bucket below authentication
	// where the subject exists; neither can be the other's slot.
	RegisterExtension(Extension{
		Name:  "ratelimit.process",
		Slot:  SlotRateLimitProcess,
		Setup: setupRateLimitProcess,
	})
	RegisterExtension(Extension{
		Name:  "ratelimit",
		Slot:  SlotRateLimit,
		Setup: setupRateLimit,
		Close: closeRateLimitStore,
	})
}

// rateLimitState memoizes the opened counter store for one startup, so the two
// frames count in one place rather than in two that cannot see each other.
var rateLimitState = struct {
	sync.Mutex
	counter RateLimitCounter
	close   func(context.Context) error
}{}

func setupRateLimitProcess(ctx context.Context) (Middleware, error) {
	config := Config[RateLimitConfig](ctx)
	if !config.Enabled || config.Process <= 0 {
		// Validation still runs through the identity frame below, so a
		// mistake in this binding is reported whichever layer is switched on.
		return nil, nil
	}
	counter, err := resolveRateLimitCounter(ctx, config)
	if err != nil {
		return nil, err
	}
	return middlewares.RateLimitProcess(config, rateLimitDeps(ctx, counter))
}

func setupRateLimit(ctx context.Context) (Middleware, error) {
	config := Config[RateLimitConfig](ctx)
	if !config.Enabled {
		releaseRateLimitStore()
		return nil, nil
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	counter, err := resolveRateLimitCounter(ctx, config)
	if err != nil {
		return nil, err
	}
	return middlewares.RateLimit(config, rateLimitDeps(ctx, counter))
}

func rateLimitDeps(ctx context.Context, counter RateLimitCounter) middlewares.RateLimitDeps {
	return middlewares.RateLimitDeps{
		Counter:  counter,
		Exempt:   rateLimitExemptions(Config[ServerConfig](ctx)),
		Reject:   writeRateLimitProblem,
		Degraded: logRateLimitDegraded,
	}
}

func resolveRateLimitCounter(ctx context.Context, config RateLimitConfig) (RateLimitCounter, error) {
	rateLimitState.Lock()
	defer rateLimitState.Unlock()
	if rateLimitState.counter != nil {
		return rateLimitState.counter, nil
	}
	counter, closer, err := openRateLimitStore(ctx, config)
	if err != nil {
		return nil, err
	}
	rateLimitState.counter, rateLimitState.close = counter, closer
	return counter, nil
}

func releaseRateLimitStore() {
	rateLimitState.Lock()
	defer rateLimitState.Unlock()
	rateLimitState.counter, rateLimitState.close = nil, nil
}

func closeRateLimitStore(ctx context.Context) error {
	rateLimitState.Lock()
	closer := rateLimitState.close
	rateLimitState.counter, rateLimitState.close = nil, nil
	rateLimitState.Unlock()
	if closer == nil {
		return nil
	}
	return closer(ctx)
}

// rateLimitExemptions are the endpoints the framework itself owns and routes.
//
// They are not a deployment setting. A readiness probe arrives from the proxy
// on the same address as every anonymous caller and would exhaust that bucket
// by itself, and one page view fetches many assets; counting either turns the
// limit into an outage on the first deploy.
func rateLimitExemptions(server ServerConfig) []string {
	exempt := make([]string, 0, 5)
	for _, path := range []string{server.Health, server.Readiness, server.OpenAPI, server.APIDoc} {
		if path = strings.TrimSpace(path); path != "" && strings.HasPrefix(path, "/") {
			exempt = append(exempt, path)
		}
	}
	if server.APIDoc != "" && server.APIDocPath != "" && strings.HasPrefix(server.APIDocPath, "/") {
		exempt = append(exempt, server.APIDocPath, strings.TrimSuffix(server.APIDocPath, "/")+"/**")
	}
	if server.Public.Enabled && strings.HasPrefix(server.Public.Mount, "/") {
		exempt = append(exempt, strings.TrimSuffix(server.Public.Mount, "/")+"/**")
	}
	return exempt
}

// writeRateLimitProblem answers a refused request through the framework error
// path, so a browser gets the application's 429 page and an API client gets a
// problem document, exactly as any other refusal does.
//
// The response names no rule and no counter. What a caller needs is when to
// come back, which the metadata already says.
func writeRateLimitProblem(w http.ResponseWriter, r *http.Request, limit pwruntime.RateLimit) {
	if err := pwruntime.ApplyProblemHeaders(w.Header(),
		pwruntime.Problem{Status: http.StatusTooManyRequests, RateLimit: &limit}); err != nil {
		Logger(r.Context()).Log(r.Context(), LevelWarn, "rate limit metadata was omitted",
			String("error", err.Error()))
	}
	if responseCommitted(w) {
		return
	}
	WriteProblem(w, r, RateLimited(limit))
}

// logRateLimitDegraded records an admission made without a working store.
//
// The request was admitted on purpose: the edge still has its own limits, and
// refusing here would convert a store incident into an outage of every limited
// route at once. Silently not limiting is the state worth knowing about.
func logRateLimitDegraded(r *http.Request, err error) {
	Logger(r.Context()).Log(r.Context(), LevelError, "rate limit admitted a request without counting it",
		String("error", err.Error()), String("path", r.URL.Path))
}
