---
id: api:data-cache
type: api
title: Data Cache
---
One generic function reaching requirement:data-result-cache through a store named in configuration, plus the store interface a backend implements.

```yaml
status: built 2026-08-13 in pwruntime and re-exported from pw, so a build on either transport reaches it
package: pw, with the stores of data:cache-store-set configured through api:runtime-configuration
names_as_built:
  acquire: MemoStore
  operate: Memo, MemoHas, MemoSet
  invalidate: MemoInvalidate, MemoInvalidateScope, MemoInvalidateTag
  observe: Stats on the handle, reporting hits, misses, coalesced waits, stale hits, and entry count together
  framing: none re-exported; a hand-written key frames its fields with cachekeybind directly, which is stdlib-only and whose helper set is wider than a copy here would stay in step with
tags:
  resolved: declared on the key type, as an optional second method beside the key one
  why_not_per_call: a tag belongs to what the entry is rather than to the moment it was read, and a per-call tag would let two writers of one entry disagree about how it is invalidated
  cost: a key type wanting tags implements two methods, and one wanting none implements one
handle:
  acquire: MemoStore, taking the context and a store name, returning the handle and an error, in the shape pw DB already resolves a configured pool from the context
  holds: no request state, so one handle serves every request and may be resolved at setup or per call, per decision:memo-store-handle
  unknown_name: an error naming the configured stores, since a passthrough would let a project believe it caches
entry_points:
  read: Memo, generic over the key type and the result type, taking the context, the handle, the key, and the fetch
  membership: MemoHas, generic over the key type, answering whether an entry is currently readable
  overwrite: MemoSet, generic over the key type and the result type, writing an entry without consulting one
  invalidate: by key, by scope, and by tag, each taking the handle
  key_type: satisfies the one-method interface of decision:cache-key-interface, which is cachekeybind's own, so a generated key method needs no adapter
  fetch: takes a context and returns the result and an error
  fetch_takes_a_context_deliberately: a closure capturing the request context would pin the shared fetch of decision:data-cache-miss-coalescing to whichever caller happened to miss first, which is the coupling that decision removes; the context handed in is the detached one
  returns: the result and an error, so a call site reads like the call it replaced
when_the_language_allows_generic_methods:
  becomes: Get, Has, and Set on the handle, with the package-level generic functions retired
  unchanged: the acquisition, the key interface, the store definitions, and every stored entry
  why_it_is_only_a_move: decision:memo-store-handle
membership_test:
  racy_by_nature: an entry may expire between the test and the read, so it answers a diagnostic or a decision to skip expensive work, never control flow assuming the next read hits
  anonymous_on_a_private_store: false, rather than testing a blank scope
  fresh_only: a stale entry answers false, because the useful question is whether the held value is current rather than whether a read would block
overwrite:
  use: a writer refreshing an entry it just made wrong, which is write-through beside invalidation
  policy_from_the_store: ttl, stale, and scope come from the definition, so a call cannot mint a longer-lived entry than the store allows
  bypasses: the fetch, and with it the coalescing, so never storing an error is the caller's to keep here
why_not_a_constructed_cache_value:
  was: a value built once per process carrying identity, ttl, scope, codec, and key function
  now: every one of those is either in the store definition or on the key type, so a caller holds a handle to a configured thing rather than a configuration
  effect: caching a call becomes wrapping it rather than declaring a field, and removing caching is deleting the wrapper
identity: decision:data-cache-entry-identity, which is the key type's own name rather than anything the call site declares
passthrough:
  cases: the section disabled, or a private store reached by an anonymous request
  behavior: call the fetch and return, so no call site branches on whether caching is on
  not_a_case: an unresolvable store name, which is an error from the acquisition rather than a quiet passthrough
invalidation:
  reach: a handler calls it after the write that made an entry wrong, since the framework does not know which read a write contradicts
  by_key: takes the same key value the read took
  by_scope: everything one scope value holds, as a key prefix
  by_tag: everything a tag names, where tags are declared beside the key
store:
  role: the backend behind one element of data:cache-store-set, not something a call site sees
  methods: get, set, delete, delete by prefix, and delete by tag
  entry: the encoded bytes, a fresh deadline, a stale deadline, and the tags it carries
  set_returns_nothing: a cache write failure must not fail a response that already fetched correctly, so an implementation reports its own failures
  concurrency: used from several goroutines at once, including the detached fetch
  bytes_not_values: the in-process backend and a remote one then hold the same thing, and no two callers are handed one mutable value; the cost is an encode round trip on every hit, which requirement:data-result-cache leaves open
  default_backend: in-process, with a TTL and an entry cap, in the shape htmlbind MemoryCache already has
codec: JSON unless the key type's package supplies another, since the value is whatever the fetch returned
errors:
  fetch: returned to the caller unchanged and never stored
  codec: a decode failure is treated as a miss and the entry dropped, because a value this build cannot read came from another one
forbidden:
  - caching a write or a transaction-local read
  - deriving a key by reflection over the key type
  - handing a caller a value another caller still holds
```
