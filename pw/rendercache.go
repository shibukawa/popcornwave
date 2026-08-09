package pw

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// The store behind the template's cache annotation.
//
// Generation already emits everything a cached component needs — its identity,
// a fingerprint of its plan, and an encoder for every declared parameter — and
// then consults a store the caller supplies. Supplying none is not a degraded
// mode: a plan with nowhere to look renders normally and computes no key, which
// is why a project using no annotation pays nothing for this file. It is also
// why the annotation did nothing at all until the option below was passed.

// renderCacheEntry is the store one configuration gets.
//
// The store outlives the request that filled it, which is the whole point, so
// it cannot be built per render. It is still keyed on the configuration rather
// than built once, so an operator who turns caching off or resizes it takes
// effect without a restart — and dropping the previous store is the eviction
// that change implies, rather than an omission.
type renderCacheEntry struct {
	config HTMLCacheConfig
	store  htmlbind.CacheStore
}

var renderCacheState atomic.Pointer[renderCacheEntry]

// renderCache returns the store this configuration renders through, or nil when
// it caches nothing.
//
// Two goroutines arriving at a cold cache can each build a store, and one of
// them is then dropped along with whatever it had already collected. That costs
// a few repeated renders once per process and needs no lock on a path every
// response takes.
func renderCache(config HTMLCacheConfig) htmlbind.CacheStore {
	if cached := renderCacheState.Load(); cached != nil && cached.config == config {
		return cached.store
	}
	entry := &renderCacheEntry{config: config}
	if config.Enabled {
		entry.store = htmlbind.NewMemoryCache(config.MaxEntries)
	}
	renderCacheState.Store(entry)
	return entry.store
}

// renderCacheOption is the option a render is given to reach the store, or nil
// when this configuration caches nothing.
//
// The tally is bound here rather than looked up per component, so the context
// is walked once per response instead of once per cached subtree.
func renderCacheOption(ctx context.Context, config HTMLCacheConfig) htmlbind.Option {
	store := renderCache(config)
	if store == nil {
		return nil
	}
	if counts := renderCacheCountsFrom(ctx); counts != nil {
		store = countingCache{store: store, counts: counts}
	}
	return htmlbind.WithCache(store)
}

// renderCacheCounts is what one response reused, reported on its render span.
//
// A hit rate is not a nicety here: the annotation carries a TTL an author
// guessed, and nothing else in the system says whether that guess was right. A
// component whose parameters differ on every call renders identically to an
// uncached one while paying for a key and a buffer, and only this tells anybody.
//
// The counters are atomic because a cached component may be called from inside
// an await boundary, which htmlbind runs in its own goroutine. The cached
// component cannot own that boundary — generation refuses it — but it can sit
// under one.
type renderCacheCounts struct {
	hits   atomic.Int64
	misses atomic.Int64
}

type renderCacheCountsKey struct{}

// withRenderCacheCounts carries a tally for the render this context covers.
// Only a traced response installs one, so an untraced render leaves the store
// unwrapped and counts nothing.
func withRenderCacheCounts(ctx context.Context, counts *renderCacheCounts) context.Context {
	return context.WithValue(ctx, renderCacheCountsKey{}, counts)
}

func renderCacheCountsFrom(ctx context.Context) *renderCacheCounts {
	counts, _ := ctx.Value(renderCacheCountsKey{}).(*renderCacheCounts)
	return counts
}

// countingCache reports what a store answered without changing what it answers.
//
// Set is passed through untouched: a write follows the miss that was already
// counted, and counting both would report every miss twice under a different
// name.
type countingCache struct {
	store  htmlbind.CacheStore
	counts *renderCacheCounts
}

func (c countingCache) Get(ctx context.Context, key string) ([]byte, bool) {
	value, ok := c.store.Get(ctx, key)
	if ok {
		c.counts.hits.Add(1)
	} else {
		c.counts.misses.Add(1)
	}
	return value, ok
}

func (c countingCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	c.store.Set(ctx, key, value, ttl)
}
