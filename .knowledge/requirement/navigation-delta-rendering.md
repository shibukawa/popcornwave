---
id: requirement:navigation-delta-rendering
type: requirement
title: Navigation Delta Rendering
---
One page URL answers a request carrying the render header with only the boundaries whose markup changed, so a search-parameter change or a same-tree navigation costs the changed region rather than a full document.

```yaml
source: decision:update-runtime-convergence
motivation: a page is a function of its request, and changing one argument should not cost a full page load; the layout that did not change is already in the browser
what_is_automatic:
  boundaries: every layout and page of a rendered chain, because a layout chain is already the shape a partial update wants
  not_a_boundary: an ordinary component call, so a five-hundred-row list does not put five hundred entries in every request
  never_a_boundary: the document shell, since a delta reuses the existing document
  identity: written by generation onto each boundary root element, under the data attribute prefix api:cli-generate sets to the Popcorn Wave brand rather than the system:tinybind default
  one_spelling: from system:tinybind v0.3.1 a render option names the async placeholder element and the boundary identifiers from that same prefix, so a document no longer carries the framework's attributes beside the module's element names
  runtime_agreement: the prefix reaches the browser as configuration, so requirement:unified-update-runtime is not rebuilt for it
  single_root: a boundary must render exactly one root element; a component that cannot is simply not a boundary and still compiles
validators:
  frame: a keyed digest of a boundary's own rendered bytes, excluding nested boundaries, and the authority for omitting one
  input: a keyed digest of the declared parameters, a cache key and a diagnostic aid
  why_two: equal inputs do not prove equal output, because a component may read the clock, the database, the locale, or the request
  execution: a delta skips transmission, never execution; only a component opting into output caching skips its own render
  keying: api:html-update-options validator_key, since an unkeyed digest of low-entropy content is guessable
handler_delta:
  required: none for a page under concept:page-tree, because api:page-render-runtime Render is the one call site and the negotiation lives there
  registered_router: a handler calling api:html-response WriteHTMLPage gets the same treatment, since both reach one render entry
  unchanged: registration, request binding, authentication, middleware order, and the generated Fragment and Params construction
mode_ordering:
  rule: the live token of api:live-delivery-protocol is tested first, then the navigation mode, then the document
  why: the shared header of decision:update-runtime-convergence means an untested live request would be answered as a complete document
  bot: api:client-classification still forces the synchronous document path of requirement:bot-synchronous-render, ahead of every update mode
response:
  operations: the outermost changed boundaries only, since a descendant of a replaced boundary is already inside the replacement
  manifest: every boundary, including unchanged ones, so the browser can rebuild its whole state from one response
  head: the tags a first-appearing component brings, installed before its markup is applied, so a region never flashes unstyled
  status: the page's real status; a delta is never a way to hide one
  headers: Vary on the render header, Cache-Control no-store, and the served mode echoed back so a proxy-substituted body is detectable
  streaming: the streamed record form, so a region applies as it is written rather than when the response ends, matching decision:automatic-async-render-selection on the document path
fallback:
  triggers: unknown mode, a protocol version the server does not speak, a truncated or stripped header, a different build, a network failure, or a body that is not a delta
  behavior: the ordinary complete document, or on the client an ordinary browser navigation; a user action is never lost
  rolling_deploy: a page rendered by the old build whose next request reaches a new one falls back cleanly, which is what the build header buys
consistency:
  within_one_response: a single consistent render of the boundaries it covers
  across_responses: not consistent; after independent updates regions may come from different points in time, which is documented rather than fixed
  fencing: a superseded response is discarded unapplied and a stale base is rejected, so out-of-order responses cannot restore older state
  repeatability: a re-render may be discarded after the server produced it, so it must be free of side effects; mutations belong in requirement:action-response-update
limits:
  - diffing is per boundary; a changed boundary is replaced whole, with no attribute or text-node diff
  - a boundary embedding a timestamp never matches and is resent every time
  - manifest size grows with boundary count, so boundaries belong at meaningful regions
  - update state lives in the browser, so nothing needs session affinity and a restart loses nothing
  - this is not a client-side framework; no component state in the browser, no virtual DOM, no client routing table
acceptance:
  - a request without the render header is byte-identical to what the page serves today
  - a search-parameter change transfers the page region and not the layout
  - the same URL serves a document to a crawler and a delta to the runtime with no cache cross-contamination
  - a request carrying another build identity receives a complete document
  - a live request on the shared header still opens a delivery stream
  - a project that never enables updates regenerates and serves identically
```
