# Change request: caching a settled async boundary, and one property to keep in the `val` error design

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.5.10
**Date:** 2026-08-14
**Status:** open

## Division of responsibility

**The capability is entirely yours; the consequences are mostly ours.** Nothing
in either ask can be approximated downstream — the refusal is a generation
check, the cache lookup happens inside plan execution, and the settled bytes
exist only where the boundary machinery is. "Why we cannot do this downstream"
below argues that rather than asserting it.

But this is not a change we would receive and be done with. Ask 2 lands on five
things this framework owns, and we list them here so the division is on the
record before either side starts:

| | Owner |
| --- | --- |
| Reporting a failed load from a template, however `val` ends up spelling it | tinybind, planned |
| Narrowing the `@cache` refusal from `await` to `live` | tinybind |
| Capturing the settled subtree, storing it, replaying it in place | tinybind |
| Never storing a boundary that failed | tinybind |
| Render-mode selection, which reads `HasAwaitBlock` and must tolerate a hit opening no boundary | **Popcorn Web** |
| The `render boundary` span, which a hit makes disappear from the trace | **Popcorn Web** |
| The cache hit and miss counters, which would stop distinguishing a markup hit from one that skipped a fetch | **Popcorn Web** |
| `html.cache.max_entries`, an entry count over entries that would now vary hugely in size | **Popcorn Web** |
| Where a newly-reportable load failure is rendered, and retiring the generated call that reports one today | **Popcorn Web** |

The fourth of ours is the one we expect to cost real work rather than a
paragraph: an entry cap chosen when an entry was a markup fragment does not
describe a store whose entries hold fetched records, and a count is the wrong
unit for that. We raise it here because it may change what you want the entry
to look like, not because we want you to solve it.

Both asks are already on your own open-question lists. This request supplies the
motivating case, the line through ask 2, and one consequence each — not a new
direction.

## Summary

`val` landed in v0.5.10 and a component can now load its own record. Writing one
immediately runs into the two limits below, and they are the same limit seen
from two sides: **a component that fetches can either report failure or be
cached, and not both.**

1. **A load bound with `val` cannot report a failure**, so a self-loading cached
   component's loader is total and renders an empty card when the record is
   missing. You have said this is being fixed, and Ask 1 asks nothing — it names
   the one property the fix needs for us to build on it.
2. **A storing `@cache` is refused on any component reaching an `await`
   boundary**, so the construct that *does* carry an error — `await` with
   `recover` — forfeits the cache. This is Ask 2, and it is the request.

Ask 1 is no longer a request: you have said a `val` error-handling change is
planned that covers it. It survives here as one property that change has to have
for us to build on it, because we are retiring a generated call on the strength
of it. **Ask 2 is the only thing this document asks you to do.**

## Ask 1 — withdrawn as a request; one question left

You have told us a `val` error-handling change is planned that covers this, so
this section is no longer asking for anything. It states the one property we
need from whatever shape it takes, because we are planning work on top of it.

The original problem: the Go implementation of `external Load(id: string):
Record` returns `Record` alone, so a lookup that fails answers with a zero value
and the template renders an empty card with no way to distinguish "no such
record" from "the database was unreachable". `requirement:render-context-externals`
carries the sharper version as an open question — the implementation can observe
cancellation and return early, and then cannot report why.

**The one question: does a failed bound expression reach the caller before the
first byte, or does it render error markup inside a response already committed
`200`?**

We have no preference between an error result on the external and a clause at
the binding site; either spelling works for us. What we cannot substitute for is
the ordering. Our discovered router generates a handler that calls a page's
loader and turns a non-nil error into a problem response *before rendering
begins*, and we intend to retire that generated call now that `val` can express
it in the template. A missing record answering `404` is the common case, and it
is the whole of what that generated call still does for us.

If the failure arrives after commit, the markup can be perfect and the status is
still `200`, so we would keep the generated rung and authors would keep writing
a Go signature to get a status. If it arrives before commit, the rung goes.

Not a request, then — just the property to keep in view while you design it, and
the reason we care which way it lands.

## Ask 2 — let `@cache` store a settled async boundary set, with `live` still refused

`requirement:component-output-cache` lists "caching a fully settled boundary
set" as an open question. We would like it taken, with one line drawn through
it that we think makes it tractable.

**`async` becomes cacheable; `live` stays refused.** The distinction is not a
compromise, it is the whole criterion:

- an `await` boundary over `external async` settles exactly once, so a settled
  form exists and there is something to store;
- an `external live` boundary keeps delivering after the document ends, so it
  never settles and there is nothing a stored byte range could be.

The flags to express this already exist. `HasLiveBlock` is documented as a
subset of `HasAwaitBlock`, so the eligibility check narrows from "reaches an
await boundary" to "reaches a live boundary" rather than gaining a new analysis.

### The semantics we are asking for

**On a miss**, the component renders exactly as it does today: placeholder,
streamed fallback, completion frame. Nothing about the first request changes.
Alongside that, the settled content of the subtree is what gets stored.

**On a hit**, the stored bytes are written in place, contiguously, with no
placeholder, no fallback, and no completion frame. The boundary is not opened at
all, because there is nothing left to wait for.

That asymmetry is the point: the cached form is *better* than the boundary, not
equivalent to it. A hit skips the wait, the client-side apply, and the render
together.

### One consequence, so you do not have to find it

`HasAwaitBlock` is a constant on the generated plan, read before rendering by
`decision:automatic-async-render-selection`. A component that may or may not
open a boundary depending on a cache hit makes that flag conservative rather
than exact: the chain reports `true`, the framework selects the streaming path,
and on a hit no boundary is ever opened.

We believe that is benign — a streamed response whose boundaries all resolve
immediately is a complete document written through the streaming path, which is
already what a page whose work finishes fast produces. But it does mean the flag
stops being a promise that a boundary *will* open, and anything relying on it
for more than path selection should be checked.

### Why we cannot do this downstream

Three separate reasons, any one of which is sufficient:

1. **The refusal is a generation check.** The eligibility list is applied where
   the annotation is compiled, and no framework option reaches it.
2. **The cache lookup is inside plan execution.** We supply a `CacheStore`
   through an option; the plan decides when to consult it. There is no seam at
   which we could say "this component would have opened a boundary — write these
   bytes instead."
3. **The settled bytes only exist inside the boundary machinery.** What reaches
   our response writer is a shell containing a placeholder plus completion
   frames addressed by boundary id. Reassembling those into the contiguous
   settled subtree means reimplementing the client's apply logic on the server,
   against a wire format that is yours and that we would then be coupled to.

## How the two interact

Once `val` can report a failure, a page that loads its own record can say the
record is missing, and a component that loads its own record can too. That is
the half you have in hand.

Ask 2 is what decides whether such a component can also be cached. Today the
only construct carrying an error through a render is `await` with `recover`, and
a storing `@cache` is refused on anything reaching an await boundary — so a
component can report failure or be cached, and not both. If the `val` fix
reports before commit, that tension eases for the page case and remains for the
component case, which is what Ask 2 is about.

There is one ordering worth naming. The router change we are planning is gated
on the `val` fix and not on Ask 2, so it can proceed as soon as the failure
arrives before commit. Ask 2 changes what those pages can then be annotated
with, which is a second step and not a blocking one.

## What we can already do today, for completeness

An `external async` whose Go implementation calls our own data cache gets the
fetch cached and keeps `recover` for errors, with no change on your side. It
works now, and it is what we will document if neither ask lands.

What it does not cache is the rendering. That is the smaller half — a hit on
markup alone saves an escape pass and a buffer — which is exactly why ask 2 is
worth asking for rather than settling here: the annotation covering both halves
is what makes one cache entry worth more than the lookup that finds it.
