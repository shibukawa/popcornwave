package pw

import (
	"context"
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// The data cache an application reaches from a handler.
//
// Every symbol here is the runtime's, re-exported so an application writes pw
// and links no transport it is not serving on. The operations are package
// functions rather than methods on the store because a Go method may not
// declare its own type parameters; when that changes they become Get, Has, and
// Set on the handle, and a call site that already resolved one does not move.

type (
	// CacheStore is a handle to one configured store, resolved by name.
	CacheStore = pwruntime.CacheStore
	// CacheKey is what a Memo key type implements: the type's identity followed
	// by the framed encoding of every field marked with the cache tag.
	//
	// It is tinybind's own interface, so a key method that generator emitted
	// satisfies it directly. A hand-written key implements the same method and
	// frames its fields with the cachekeybind helpers; nothing is re-exported
	// here, because that package is stdlib-only and its helper set is wider
	// than a copy here would stay in step with.
	CacheKey = pwruntime.CacheKey
	// CacheTagger is the optional half of a key type, naming the tags whose
	// invalidation drops the entry.
	CacheTagger = pwruntime.CacheTagger
	// CacheStats is what one store has answered.
	CacheStats = pwruntime.CacheStats
)

// MemoStore resolves a configured store by name.
//
// A disabled cache section returns no store and no error, and every operation
// on a nil store falls through to its fetch, so a deployment removes caching
// without editing a call site. An unconfigured name is an error naming what is
// configured.
func MemoStore(r *http.Request, name string) (*CacheStore, error) {
	return pwruntime.MemoStore(r.Context(), name)
}

// MemoStoreContext is MemoStore for code below the handler, and for a
// resolution done once at startup rather than per request.
func MemoStoreContext(ctx context.Context, name string) (*CacheStore, error) {
	return pwruntime.MemoStore(ctx, name)
}

// Memo returns what fetch produced for this key, reusing a stored result while
// it is fresh and coalescing concurrent misses onto one fetch.
//
// The fetch receives a context detached from every waiter, which is what lets
// one request's cancellation leave the shared work alone. Do not capture the
// request context inside the closure instead: that would pin the fetch to
// whichever caller happened to miss first.
func Memo[K CacheKey, T any](ctx context.Context, store *CacheStore, key K, fetch func(context.Context) (T, error)) (T, error) {
	return pwruntime.Memo[K, T](ctx, store, key, fetch)
}

// MemoHas reports whether this key currently has a fresh entry; a stale one
// answers false. It is racy by nature, so it answers a diagnostic or a decision
// to skip expensive work rather than control flow assuming the next read hits.
func MemoHas[K CacheKey](ctx context.Context, store *CacheStore, key K) bool {
	return pwruntime.MemoHas[K](ctx, store, key)
}

// MemoSet writes an entry without consulting one, which is how a writer
// refreshes what it just made wrong. The lifetime comes from the store.
func MemoSet[K CacheKey, T any](ctx context.Context, store *CacheStore, key K, value T) error {
	return pwruntime.MemoSet[K, T](ctx, store, key, value)
}

// MemoInvalidate drops one entry, taking the key the read took.
func MemoInvalidate[K CacheKey](ctx context.Context, store *CacheStore, key K) {
	pwruntime.MemoInvalidate[K](ctx, store, key)
}

// MemoInvalidateScope drops everything one reader holds.
func MemoInvalidateScope(store *CacheStore, scope string) {
	pwruntime.MemoInvalidateScope(store, scope)
}

// MemoInvalidateTag drops everything a tag names, which is the axis the
// reader-first key layout cannot serve as a prefix.
func MemoInvalidateTag(store *CacheStore, tag string) {
	pwruntime.MemoInvalidateTag(store, tag)
}
