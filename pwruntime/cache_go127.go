//go:build go1.27

package pwruntime

import "context"

// The data cache operations as methods on the store handle.
//
// decision:memo-store-handle put the handle in place before these could exist,
// so the line that acquires a store is already written the way it stays and
// only the operation moves. Each method is the matching package function with
// the store as the receiver, and the function keeps the body and the full
// documentation until the day the functions are retired — so nothing here can
// drift from what it delegates to.
//
// The file is tagged go1.27 because a method may not declare its own type
// parameters before then, and two things follow from tagging it rather than
// waiting. An older toolchain does not see the file, so today's build is
// unchanged down to the object code. And the tag names a release rather than a
// feature — methods with type parameters are not committed to 1.27 — so if that
// release arrives without them, this tag moves to the one that carries them.
// The same tag gates the TinyGo build, which reports the release tags of the Go
// it accepts, so neither build takes the new spelling before the other.
//
// A nil handle is the disabled deployment, and every method below falls through
// exactly as its function does: a nil pointer is a legal receiver, and the nil
// check is in the body being called.

// Get returns what fetch produced for this key, reusing a stored result while
// it is fresh and coalescing concurrent misses onto one fetch.
//
// It is Memo with the store as the receiver, and that function's rule about the
// fetch context holds unchanged: the fetch receives a context detached from
// every waiter, so do not capture a request context inside the closure instead.
func (s *CacheStore) Get[K CacheKey, T any](ctx context.Context, key K, fetch func(context.Context) (T, error)) (T, error) {
	return Memo[K, T](ctx, s, key, fetch)
}

// Has reports whether this key currently has a fresh entry; a stale one answers
// false. It is racy by nature, so it answers a diagnostic or a decision to skip
// expensive work, never control flow assuming the next read hits.
func (s *CacheStore) Has[K CacheKey](ctx context.Context, key K) bool {
	return MemoHas[K](ctx, s, key)
}

// Set writes an entry without consulting one, which is how a writer refreshes
// what it just made wrong. The lifetime comes from the store.
func (s *CacheStore) Set[K CacheKey, T any](ctx context.Context, key K, value T) error {
	return MemoSet[K, T](ctx, s, key, value)
}

// Invalidate drops one entry, taking the key the read took.
func (s *CacheStore) Invalidate[K CacheKey](ctx context.Context, key K) {
	MemoInvalidate[K](ctx, s, key)
}

// InvalidateScope drops everything one reader holds.
//
// It declares no type parameter and could have been a method all along. It
// moves with the rest so that the store reads one way rather than two.
func (s *CacheStore) InvalidateScope(scope string) {
	MemoInvalidateScope(s, scope)
}

// InvalidateTag drops everything a tag names, which is the axis the
// reader-first key layout cannot serve as a prefix. Like InvalidateScope it
// needs no type parameter and moves for the company rather than the language.
func (s *CacheStore) InvalidateTag(tag string) {
	MemoInvalidateTag(s, tag)
}
