# Change request: a scope on the cache annotation, defaulting to private

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.4.8
**Date:** 2026-08-09
**Status:** open

## Summary

`decision:cache-key-derivation` closes with a hard rule: *request, session, and locale variation must be declared parameters or the component must not be cached.* It is the right rule, and it is why a component that reads per-user state anywhere but its parameter list has no cached form available to it at all.

We would like that to gain an option instead of staying a prohibition. Three parts, which only make sense together:

1. **A `scope` option on `@cache`, defaulting to `private`.** A private component's key is prefixed with a scope value the caller supplies per render; a `public` one keys on parameters exactly as today.
2. **The declaration readable before the render starts,** folded over the call graph and over a chain, the way `HasAwaitBlock` and `Vary` already are.
3. **Generation refusing `@cache(scope: "public")` on a component whose call graph reaches a declared private one,** with the declaration position, the way `no_await` already refuses.

One companion in the same shape: `Plan.Vary` is declared, folded, and readable, but does not reach the cache key either. `options.future` parks that interaction as unspecified; we would like it decided.

**The scope value is ours and opaque to you.** We pass a unique-per-user identifier with every render, as part of the same option set that carries the store. You never learn what it means.

## Why the default is private

Our own deployments are login-gated: almost every page sits behind `policy:authenticated-path-protection`, and the shared-cacheable page is the exception. That is our context, not an argument. The argument is the failure asymmetry.

A component treated as public that is actually per-user serves one user's rendered output to another. A component treated as private that is actually shared costs a cache miss. Those are not comparable, and the undeclared state should sit on the side where forgetting is slow rather than wrong.

The second reason is what static analysis cannot see. A component's call graph is fully visible to generation — template component references are static tags, so there is no dynamic dispatch to lose — but an `external` Go function is opaque. A component calling an external function that reads the request identity out of `ctx` looks shared to every check either of us can write. Under a public default it is silently shared. Under a private default it is correct without anyone noticing there was a question.

## Ask 1 — `scope` on the annotation

```
@cache(ttl: "5m", scope: "public")
export component ProductList(rows: Product[]): html { ... }

@cache(ttl: "30s")
export component AccountSummary(plan: string): html { ... }
```

| `scope` | key |
| --- | --- |
| `private` (default) | scope value + `ID` + framed parameters |
| `public` | `ID` + framed parameters — today's behaviour |

`CachePolicy` carries it as one field:

```go
type CachePolicy[P any] struct {
	ID     string
	TTL    time.Duration
	Key    func(P) string
	Scoped bool  // key is prefixed with the render's scope value
}
```

`cacheKey` prepends the framed scope value when `Scoped`, using the existing framing so a scope value cannot spell out another key. `decision:reflection-free` is untouched: this is one string in front of a string that generated code already builds.

The value arrives as an option beside `WithCache`:

```go
htmlbind.WithCacheScope(scope string)
```

**We supply a unique-per-user identifier on every render that can reach a private component.** On our side that is a single value every authentication method converges on — session login, passkey, and bearer token all resolve to one local account identifier before any handler runs — so the guarantee is structural rather than a convention we are promising to keep. It is an opaque string to you: never an email, never a token, never a session identifier.

The one behaviour we cannot decide for you is the absent case. **A private component rendered with no scope value must store nothing** — not under an empty scope, which would be a shared entry wearing a private label. That is the fallback rather than the design; we do not expect it to fire, and we would rather it be a miss than a decision.

## Ask 2 — readable before the first byte

This is the part we cannot work around downstream, and the reason is timing rather than taste.

`Cache-Control` has to be on the wire before the first byte of a streamed document. A private component sitting four levels down renders long after that. Any signal we compute *during* a render is therefore available only on the buffered branch, which would mean a response's cache policy depending on whether `Streaming` was on — a configuration switch silently moving a security-relevant header.

So the declaration has to fold upward statically, and the machinery is already built for two other properties:

- `Plan.HasAwaitBlock` is computed over the call graph at generation, so *this component or any component it calls* is one bit.
- `foldSlots` (`htmlbind/plan.go:253`) unions `hasAwait`, `hasLive`, `head`, `assets`, and `vary` from slot-bound fragments at `Bind` time — before any rendering.
- `HasAwaitBlock(wrappers, leaf)` and `MergeVary(wrappers, leaf)` union across a chain for the document/layout/page shape.

We are asking for the same path: a `Plan.Private` bit computed over the call graph, unioned in `foldSlots`, exposed as `Fragment.Private()` and `Wrapper.Private()`, and unioned across a chain by an `IsPrivate(wrappers, leaf)` beside the two helpers above.

The chain union is what makes this usable. A single declaration on an authenticated layout covers every page beneath it, which is the shape a login-gated application actually has — one annotation rather than one per page.

For diagnostics we would like the courtesy `HeadSources` provides: when a chain reports private, a way to name the component that declared it. `HeadSources` exists because *a caller that cannot deliver a head contribution uses it to report which component to change*, and the situation is identical — an author who wrote `public` and got private needs to know where to look.

## Ask 3 — refuse public over private at generation

`@cache(scope: "public")` on a component whose call graph reaches a **declared** private one is a generation error, reported at the declaration position.

An error rather than a warning, because the combination does not merely disagree with itself — **the parent's entry voids the child's declaration entirely.** Your own execution rule is what makes it so: *hit: write stored bytes into the current stream; the component body does not run.* A public parent that hits never runs the private child, so whichever user rendered the parent first has their private output baked into a scope-free entry and served to everyone after them.

That is the normal path on the first request, not a race and not an edge case. One miss assembles the bytes, and every hit afterwards distributes them.

There is also no source to protect. `scope` does not exist yet, so no template can express this combination today, and the usual argument for a warning — do not break what already builds — has nothing to apply to.

Declared, not merely undeclared. An ordinary component with no annotation must not block a public assertion, or nothing could ever be public — the author's `public` is an assertion about the subtree, and an undeclared component inherits it. Only an explicit `private` contradicts it.

The check is affordable: it is the call-graph walk `HasAwaitBlock` already performs, reading a different bit. And `await_rationale.v1` already settled this shape for the neighbouring case — *reject at generation time with the declaration position, instead of silently caching only the initial pass.*

We would still keep a runtime downgrade to private on our side as a fail-safe. It is sound in isolation — a scoped parent keys the child correctly — but it is the wrong answer at generation time, where the author asked for a public entry and would silently receive none. If it ever fires it means the generation check has a hole, so we would log it as an error rather than treat it as normal operation.

### The other direction is fine

A private component containing a public one needs no diagnostic. The public child is stored under its own unscoped key exactly as declared, and its bytes also sit inside the parent's scoped entry — a shared thing inside a private container, with no direction in which anything leaks.

The only cost is duplicated storage, and only for the calls that arrive through this parent. A public component usually has other callers, where its own entry is what serves them.

## Companion — vary and the cache key

`requirement:builtin-element-registration` gives the vary axis two reasons to exist, and the second is a cache reason: *the caller cannot build a Vary header for what it cannot see, and an output cache cannot refuse to store what it cannot key.*

But `cacheKey` is `KeyString(c.ID) + c.Key(params)`, the eligibility list in `decision:cache-component-declaration` says nothing about vary, and `options.future` parks the interaction as unspecified rather than reserved. So a component reaching a builtin element that reads a cookie is cacheable today, and the cookie does not enter its key.

We raise it here because it is the same defect Ask 1 addresses — a declared dependency that does not reach the key — and because whatever you decide for scope probably decides this too. Either the axis folds into the key, or a component with a non-empty `Plan.Vary` is ineligible for `@cache`. We have no preference between those; we would like the interaction to stop being unspecified.

*(The consumer side of `MergeVary` is ours and missing: we never wired it into our response path, so a declared vary axis reaches no `Vary` header today. That is our bug and we are fixing it in the same change. We mention it only so you know the accessor has a caller now.)*

## Migration

The default flip is a behaviour change for every existing `@cache`, and it does not announce itself. Today's annotation keys on parameters alone, which is `scope: "public"` under the new spelling. Under a private default those components become per-user keyed, and the observable result is not an error — it is a cache that quietly stops being reused, on exactly the components an author declared most reusable.

We would rather that be a compile error than a performance cliff found in production. Our preference, in order:

1. **Require `scope` explicitly for one minor version.** `@cache` without it fails generation, naming both spellings. Every existing call site is touched once, deliberately, and the author decides per component. Then default to `private` in the version after.
2. Default to `private` immediately behind a major bump, with the change named in the release notes.

We would take either. We would not take a silent default flip in a patch release, and we would rather not have a permanent required argument — the whole point of the default is that forgetting it is safe.

## What we are not asking for

- **Any knowledge of identity, sessions, or authentication.** The scope value is an opaque string we supply per render. You never learn what it means, and nothing about how we derive it is your concern.
- **An HTTP header.** Turning `IsPrivate` into `Cache-Control` is ours, alongside the policy we already write for the update, redraw, and sequence responses.
- **A shared or network-backed store.** `api:cache-store` is right as it is; scoping happens in the key, above the store interface, so an existing adapter needs no change.
- **A relaxation of the `@cache` eligibility rules.** Single root, no await, no nested boundary, no html parameters, no shell all stay. Scope is a bit on a component that already qualifies.
- **A configuration switch.** We are not adding one on our side either. A knob that flips a security default across a whole deployment gets flipped once during an investigation and never flipped back, and it would make the annotation mean something different in two places. The declaration is the only control.
- **Enforcement we know is impossible.** `private` is a declaration, not a proof. A component whose Go body reads per-user state from `ctx` and declares nothing stays wrong, and neither of us can catch it. We are asking for a default that makes the common case safe, not a guarantee.

## Compatibility

Every item is additive except the default, which Migration covers.

- `CachePolicy.Scoped` is a new field on a struct only generated code builds.
- `WithCacheScope` is a new option; a render that omits it behaves as today for public components and stores nothing for private ones.
- `Plan.Private`, `Fragment.Private()`, `Wrapper.Private()`, and `IsPrivate` are additive, and a caller that reads none sees no change.
- The Ask 3 refusal only rejects a combination that cannot be written today, since there is no `scope` to write.
- The vary companion is a decision, not necessarily a code change; if it lands as an eligibility rule it rejects a combination that is currently mis-keyed.

## What we can contribute

- **A consumer immediately.** We have the response path that needs `IsPrivate`, the identity that feeds `WithCacheScope`, and a login-gated example application to run it against. We can implement against a prerelease and report where the declaration is under-determined, as we did for the v0.4.7 update surface.
- **The layered model this fits into.** Our `policy:layered-cache` already names *private scope* as one of four cache declarations and states the rule this implements — *private keys contain user, session, or tenant scope* — so the vocabulary exists on our side and we can align on it rather than invent a second one.
- **Measurements.** We can report what the private default costs in hit rate on real pages, which is the number that should decide whether option 1 or option 2 in Migration is right.

## Related concepts

**Yours:** `requirement:component-output-cache`, `decision:cache-component-declaration`, `decision:cache-key-derivation`, `api:cache-store`, `requirement:builtin-element-registration`, `data:component-render-capabilities`, `rule:component-capability-combinations`, `decision:reflection-free`, `requirement:chain-render-pipeline`

**Ours:** `policy:layered-cache`, `policy:authenticated-path-protection`, `policy:security-response-headers`, `concept:session-storage-boundary`
