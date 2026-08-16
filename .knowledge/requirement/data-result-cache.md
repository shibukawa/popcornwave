---
id: requirement:data-result-cache
type: requirement
title: Data Result Cache
---
Reuse what a fetch returned for equal typed arguments until it expires, coalescing concurrent misses and accepting explicit invalidation.

```yaml
status: built 2026-08-13 in pwruntime with the memory backend, every acceptance below covered by a test
source: policy:layered-cache, whose data layer declared this and named no store
covers: what a handler or an api:typed-external-function fetches — an upstream API call, a generated read — and never rendered bytes
complement:
  component_cache: requirement:component-output-cache saves the rendering and never the fetching; this is the other half of the same response
  shared: key framing, the scope prefix, and the private default of policy:component-cache-scope
  not_shared: the store interface, the entry, the identity, and coalescing, each for a reason recorded below
value: one encoded result for one argument set
store:
  interface: api:data-cache Store, a superset of the htmlbind CacheStore shape the render cache uses
  why_not_that_one: tag invalidation needs a reverse index and a stale window needs a second deadline on the entry; neither fits Get and Set over bytes with one ttl, and widening the upstream interface would widen it for the render path that needs neither
  instances: data:cache-store-set, named and configured like a database pool, so ttl, stale, scope, cap, and backend are operational rather than written at the call site
  why_separate_from_html_cache: max_entries is one number for the whole process, chosen for render bytes, and requirement:component-output-cache already thrashes it once a private component joins; API payloads are larger and arrive on a different schedule
  absent: the section disabled means the fetch runs every time, so the cache stays a deployment choice rather than a rewrite
key:
  shape: scope, identity, then the framed encoding of every argument
  framing: the cachekeybind helpers — each value written as its byte length, a colon, then the value, so a concatenation splits exactly one way and two argument lists cannot reach one key; the set is wider than htmlbind's, covering every integer and float width
  identity: decision:data-cache-entry-identity, carried by the key type rather than by the call site
  carried_by: decision:cache-key-interface — the key type implements one method, emitted by the cachekeybind generator of system:tinybind v0.5.9 from `cache:"key"` marked fields
  marking_is_opt_in: only a marked field is in the key, so an entity struct can be passed as-is with the query picked out of it; upstream refused the default-include this framework asked for, because an entity's fields are mostly the result and default-include would build the key from the value the lookup exists to avoid fetching
  what_opt_in_leaves_open: a new dependency added and left unmarked is still silently absent from the key, which generation no longer closes; it does close the identity prefix and the framing, both wrong-answer failures
  no_reflection: the same constraint api:typed-external-function already states; deriving a key by walking a value would also hide which fields the entry depends on, and that set is what the safety rules below are about
  external_declarations: an api:typed-external-function already carries its argument types, so the same generator could emit a key type for one; deferred, and that concept's cache line is where it lands
scope_value:
  default: private, prefixed with the pwruntime RequestAuthentication Subject, the value the component cache already scopes on
  why_that_default: a shared entry holding one reader's data is the failure that does not degrade, and a private entry that could have been shared only costs hits
  anonymous: no subject stores nothing, because an entry written under a blank identity is a shared entry wearing a private label
  public: declared per cache, and only where the result is a function of the declared arguments and nothing else
behavior:
  hit: decode and return; the fetch does not run
  miss: decision:data-cache-miss-coalescing
  expiry: an expired entry is a miss unless a stale window covers it
  stale: inside the stale window the held value returns immediately and one revalidation starts, coalesced by the same mechanism a miss uses
  errors: a failed revalidation leaves the stale entry in place until its stale deadline, which is the whole point of configuring one
invalidation:
  why: a write invalidates a read, and a TTL alone leaves open exactly the window the writer already knows is wrong
  by_key: exact, from the same key function the read uses
  by_scope: the scope is prepended, so everything one reader holds is one prefix
  by_tag: declared on write, because the scope-first layout makes per-identity-across-readers a scan rather than a prefix, and swapping the order to fix that would break the reader prefix instead
  ordering_is_a_tradeoff_not_an_oversight: one axis can be a prefix and the other cannot; the reader axis is the one a deletion request actually arrives for
safety:
  - never cache an error or a partial result
  - never cache a write, and never cache a read taken inside a transaction
  - cache only a result whose whole dependency set is in the key; an argument the fetch reads from the context and the key does not encode is a wrong answer rather than a stale one
  - a decoded value belongs to its caller, so the store retains no slice it handed out
observability:
  built: the four counters live on the store and are read together through its Stats accessor, so a test and a diagnostic can both see them
  not_built: nothing reaches a span yet, so requirement:modern-observability still reports this layer uninstrumented
  span_when_it_lands: a fetch that runs opens the client span api:typed-external-function already opens; a detached revalidation cannot parent to a request that may have ended, so it links instead
acceptance:
  - equal arguments inside the TTL run the fetch once
  - changed arguments, a changed identity, or an expired entry with no stale window cannot reuse a value
  - concurrent misses on one cold key run the fetch once, including a caller that missed just as the previous fetch deregistered
  - a fetch every waiter abandoned still fills the entry, since the fetch stores its own result rather than the waiter storing it
  - a waiter that cancels stops waiting without stopping the fetch
  - a disabled section behaves exactly like calling the fetch directly, with no call site edited
  - two key types holding equal field values do not reach one entry
  - resolving an unconfigured store name fails rather than silently never caching
  - moving the typed operations onto the store handle later edits no call site, per decision:memo-store-handle
open_questions:
  - which backend ships beside memory, which is what decides whether the codec is negotiable and forces the unstamped-build partition of decision:data-cache-entry-identity
  - a typed in-memory entry skipping the encode round trip, which should be measured before it is designed
  - reaching a span, which is the half of observability the build left undone
  - whether the tag half should be generated too, since cachekeybind emits the key method and CacheTags is still hand-written
closed_by_the_build:
  tags_are_declared_on_the_key_type: as an optional second method, per api:data-cache; a tag belongs to what the entry is rather than to the moment it was read
  a_detached_fetch_is_bounded_by_the_store: the fetch timeout of data:cache-store-set, which the coalescing decision required and the first design had not carried
```
