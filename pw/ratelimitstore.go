package pw

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/shibukawa/popcornwave/middlewares"
)

// RateLimitCounter is the storage a limiter counts in: add one to the count
// under a key for a window, and report the new total.
type RateLimitCounter = middlewares.RateLimitCounter

// Rate limit backend names a deployment selects with ratelimit.backend.
const (
	// RateLimitBackendMemory counts inside this process, and is built in.
	RateLimitBackendMemory = middlewares.RateLimitBackendMemory
	// RateLimitBackendRedis counts in a shared server. It needs the blank
	// import of ratelimitstore/redis.
	RateLimitBackendRedis = middlewares.RateLimitBackendRedis
)

// DefaultRateLimitKeyPrefix namespaces the keys a shared counter store owns.
const DefaultRateLimitKeyPrefix = middlewares.DefaultRateLimitKeyPrefix

// RateLimitStoreFactory opens the counter store a configuration selects. The
// returned close runs during shutdown and may be nil.
type RateLimitStoreFactory func(ctx context.Context, config RateLimitConfig) (RateLimitCounter, func(context.Context) error, error)

var rateLimitStores = struct {
	sync.RWMutex
	factories map[string]RateLimitStoreFactory
}{}

// RegisterRateLimitStore registers factory under name. A storage plugin calls
// it from init, so a blank import is what puts a backend in a binary:
//
//	import _ "github.com/shibukawa/popcornwave/ratelimitstore/redis"
//
// The in-process counter is built in, because it adds no dependency and a
// limiter that needs one before it starts is one nobody switches on.
//
// A duplicate or empty name panics: two backends answering one configuration
// value is a build mistake, not a runtime condition.
func RegisterRateLimitStore(name string, factory RateLimitStoreFactory) {
	if name == "" || factory == nil {
		panic("pw: rate limit store needs a name and a factory")
	}
	rateLimitStores.Lock()
	defer rateLimitStores.Unlock()
	if rateLimitStores.factories == nil {
		rateLimitStores.factories = make(map[string]RateLimitStoreFactory)
	}
	if _, taken := rateLimitStores.factories[name]; taken {
		panic(fmt.Sprintf("pw: rate limit store %q is already registered", name))
	}
	rateLimitStores.factories[name] = factory
}

// RateLimitStores lists the registered backend names in order, which is what
// an error message reports when a configuration names one that is missing.
func RateLimitStores() []string {
	rateLimitStores.RLock()
	defer rateLimitStores.RUnlock()
	names := make([]string, 0, len(rateLimitStores.factories))
	for name := range rateLimitStores.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func init() {
	RegisterRateLimitStore(middlewares.RateLimitBackendMemory,
		func(context.Context, RateLimitConfig) (RateLimitCounter, func(context.Context) error, error) {
			return middlewares.NewMemoryRateLimitCounter(), nil, nil
		})
}

// openRateLimitStore resolves the configured backend, naming the blank import
// a deployment is missing rather than only reporting an unknown value.
func openRateLimitStore(ctx context.Context, config RateLimitConfig) (RateLimitCounter, func(context.Context) error, error) {
	name := config.Backend
	if name == "" {
		name = middlewares.RateLimitBackendMemory
	}
	rateLimitStores.RLock()
	factory, ok := rateLimitStores.factories[name]
	rateLimitStores.RUnlock()
	if !ok {
		return nil, nil, fmt.Errorf("ratelimit.backend %q is not registered; linked backends are %v", name, RateLimitStores())
	}
	return factory(ctx, config)
}
