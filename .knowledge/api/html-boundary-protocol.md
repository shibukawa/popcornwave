---
id: api:html-boundary-protocol
type: api
title: HTML Boundary Wire Protocol
---
system:tinybind emits boundary placeholders and allocates their identity; every completion framing and the client runtime that acts on it belong to Popcorn Web.

```yaml
upstream_emits:
  placeholder: "<tb-boundary id=\"tb-N\" style=\"display:contents\">fallback</tb-boundary>", where the element name and the id prefix follow the configured boundary prefix from system:tinybind v0.3.1 and carry the Popcorn Web brand here
  commit: shell, merged head, and every fallback flush before slow work settles
  head: component contributions only, with nothing injected
  completion: htmlbind.Content carries BoundaryID and HTML; WriteTo writes the bare fragment and nothing else
  order: settle order, not document order
baseline: v0.1.20 removed the template, the marker, and the runtime script from the module
framework_owns:
  - the framing written around each fragment, on every transport
  - the client runtime that applies a fragment to its placeholder
  - keeping framing and runtime consistent, since they are one design
  narrowed_at_v0_3_0: the fetch envelope and its runtime half come from upstream, so what stays framework-owned is the parser and record envelopes, the merged asset of requirement:unified-update-runtime, and the naming; decision:update-runtime-convergence records why
envelopes:
  parser_driven:
    use: the streaming initial load, where the browser parser consumes the response as it arrives
    framing: "<template data-tb-boundary=\"tb-N\">fragment</template><tb-apply for=\"tb-N\"></tb-apply>"
    trigger: the tb-apply marker, never the template element
    rationale: an HTML parser inserts an element at its start tag, so reacting to the template can read an unfinished one and replace the placeholder with nothing, losing the fallback along with the result
    evidence: observed once the template start tag landed in its own network chunk; invisible in development, and surfacing only once a proxy, TLS record, or compressing encoder splits the bytes
    invariant: the marker follows the closing template tag in the byte stream, so it cannot exist before its template is complete, however the bytes were chunked
    ownership_note: the module no longer enforces this, so the framing and the runtime must preserve it deliberately
  document_replacement:
    use: decision:unhandled-boundary-escalation, where a page is replaced rather than patched
    framing: an inert template holding the error document followed by its own marker, carrying no boundary id
    target: the children of the document body, so no identifier is needed
    trigger: the marker, for the same parser reason as a completion
    terminal: nothing may be written after it, and the runtime ignores any later completion
  record_driven:
    use: api:live-delivery-protocol, where deliveries continue after the document is complete
    framing: one JSON record per delivery from htmlbind.Content.AppendJSON, never markup
    rationale: past the initial document there is no parser reading, so the marker rule has nothing to trigger
  fetch_driven:
    use: flow:partial-refresh and any navigation the runtime drives with fetch
    framing: boundary id and HTML as data, never marker markup
    trigger: the runtime applies by id once it holds the complete bytes, because a resolved fetch already proves completeness
    rationale: no parser runs over a fetched body, so a marker has nothing to connect; parsing one into a detached document does not upgrade custom elements at all, and adopting the nodes later upgrades them at an unpredictable point
    supplied_by: system:tinybind htmlupdate from v0.3.0, per decision:update-runtime-convergence, so the delta encoder, the operation kinds, and the manifest codec are no longer this framework's to define
    operations: replace for a boundary, plus head operations installing a first-appearing component's tags before its markup lands
    addressing: the framework boundary attribute first, then the author-written element id, since requirement:action-response-update and requirement:reloadable-component-endpoint address regions the author named
    streamed_form: one record per line, each carrying its own manifest entry because a trailing manifest cannot precede the operations it describes, and an explicit terminator because that is the only way to tell a finished render from a truncated one
rules:
  - marker framing appears only in a parser-driven response body
  - the runtime core is a plain apply function taking a boundary id and HTML
  - a custom element, where used, is a thin parser-path adapter over that function
  - apply each boundary at most once per delivery; a settle-once boundary therefore applies once, and a live one applies per delivery into the range it already holds
  - do nothing when the target placeholder is missing and no range holds that id, so a truncated or superseded response leaves the fallback visible
  - no framing carries script, so policy:security-response-headers needs no nonce beyond the shell reference
cleanup:
  on_apply: the placeholder element and the framing elements are removed, and the applied content is left bracketed by a pair of comment nodes carrying the boundary id
  why_a_range: a live boundary is re-rendered for as long as its subscription lives, so its content needs an address that survives the first delivery; comments are inert, invisible to CSS and layout, and bracket a range rather than wrap it, so a delivery of several top-level nodes needs no container element the author never wrote
  why_every_boundary: nothing on the wire says which boundary is live, since the placeholder markup and the delivery record are identical for both kinds, so the range is kept for all of them
  effect: no id, template, or marker element survives an apply, and applied boundaries still cannot collide with anything later
  cost: two comment nodes per applied boundary, which is the price of the address a live delivery replaces
  not_unconditional: a boundary that never settles keeps its placeholder on purpose, because that placeholder is the visible fallback
  nested_reuse: an enclosing live boundary re-rendering removes the ranges inside it, and the replacement subtree carries those boundaries' placeholders again under the same ids, so the next delivery re-establishes them
identifiers:
  scope: allocated upstream; a top-level boundary is tb-1, tb-2
  positional_since_v0_2_7:
    shape: each boundary's subtree is a namespace under its own id, so a nested boundary is tb-1-1 rather than a number from one flat counter
    gained: the same chain executed again produces the same ids, which is what requirement:live-connection-recovery reconnects against, and a nested id no longer depends on the order siblings settled in
    breaking: a project with a boundary nested inside another sees different ids than before
    affects_here: the fetch-path rewrite below namespaces whole ids, so it keeps working, but any code matching the flat tb-N shape does not
  cleared_by_apply: an applied boundary removes its own id, so history is not what collides
  remaining_collisions:
    unsettled: a boundary that timed out or was cut off keeps its placeholder forever, and a later render numbering from tb-1 duplicates that id
    superseded: completions from a response still in flight carry ids the next response reuses
    concurrent: two refreshes rendered at once both number from tb-1
  resolution:
    parser_path: no rewrite needed; an initial load parses into a fresh document
    fetch_path: the runtime rewrites placeholder ids to a per-response namespace as it inserts a fragment, and maps arriving completion ids through the same rename
    why_it_works: the framework owns both the framing and the insertion on this path, so it holds the bytes before they reach the document
    scope_of_rewrite: every inserted fragment, since a completion can carry nested boundaries of its own
  upstream_change: not required, so document-lifetime identity stays a simplification rather than a blocker
rejected_alternative:
  what: read every boundary through fetch, including the initial load, to have one application mechanism
  why: the boundaries could not flush until the client asked for them, which forfeits the single-response property of flow:initial-streaming-render
transport_boundary: htmlbind writes no headers, sets no status, and chooses no encoding
```
