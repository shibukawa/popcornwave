package pw

import (
	"context"

	"github.com/shibukawa/popcornwave/pwratelimit"
)

// The counter store registry is the shared leaf's, so a backend registered by a
// blank import serves whichever transport this binary was built for. What is
// here is the pw spelling of it, unchanged.

// RateLimitCounter is the storage a limiter counts in: add one to the count
// under a key for a window, and report the new total.
type RateLimitCounter = pwratelimit.Counter

// Rate limit backend names a deployment selects with ratelimit.backend.
const (
	// RateLimitBackendMemory counts inside this process, and is built in.
	RateLimitBackendMemory = pwratelimit.BackendMemory
	// RateLimitBackendRedis counts in a shared server. It needs the blank
	// import of ratelimitstore/redis.
	RateLimitBackendRedis = pwratelimit.BackendRedis
)

// DefaultRateLimitKeyPrefix namespaces the keys a shared counter store owns.
const DefaultRateLimitKeyPrefix = pwratelimit.DefaultKeyPrefix

// RateLimitStoreFactory opens the counter store a configuration selects. The
// returned close runs during shutdown and may be nil.
type RateLimitStoreFactory = pwratelimit.StoreFactory

// RegisterRateLimitStore registers factory under name. A storage plugin calls
// it from init, so a blank import is what puts a backend in a binary:
//
//	import _ "github.com/shibukawa/popcornwave/ratelimitstore/redis"
//
// The in-process counter is built in, because it adds no dependency and a
// limiter that needs one before it starts is one nobody switches on.
func RegisterRateLimitStore(name string, factory RateLimitStoreFactory) {
	pwratelimit.RegisterStore(name, factory)
}

// RateLimitStores lists the registered backend names in order, which is what
// an error message reports when a configuration names one that is missing.
func RateLimitStores() []string { return pwratelimit.Stores() }

func openRateLimitStore(ctx context.Context, config RateLimitConfig) (RateLimitCounter, func(context.Context) error, error) {
	return pwratelimit.OpenStore(ctx, config)
}
