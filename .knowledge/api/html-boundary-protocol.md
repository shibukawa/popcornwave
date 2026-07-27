---
id: api:html-boundary-protocol
type: api
title: HTML Boundary Wire Protocol
---
system:tinybind emits boundary placeholders and allocates their identity; every completion framing and the client runtime that acts on it belong to Popcorn Wave.

```yaml
upstream_emits:
  placeholder: "<tb-boundary id=\"tb-N\" style=\"display:contents\">fallback</tb-boundary>"
  commit: shell, merged head, and every fallback flush before slow work settles
  head: component contributions only, with nothing injected
  completion: htmlbind.Content carries BoundaryID and HTML; WriteTo writes the bare fragment and nothing else
  order: settle order, not document order
baseline: v0.1.20 removed the template, the marker, and the runtime script from the module
framework_owns:
  - the framing written around each fragment, on every transport
  - the client runtime that applies a fragment to its placeholder
  - keeping framing and runtime consistent, since they are one design
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
  fetch_driven:
    use: flow:partial-refresh and any navigation the runtime drives with fetch
    framing: boundary id and HTML as data, never marker markup
    trigger: the runtime applies by id once it holds the complete bytes, because a resolved fetch already proves completeness
    rationale: no parser runs over a fetched body, so a marker has nothing to connect; parsing one into a detached document does not upgrade custom elements at all, and adopting the nodes later upgrades them at an unpredictable point
rules:
  - marker framing appears only in a parser-driven response body
  - the runtime core is a plain apply function taking a boundary id and HTML
  - a custom element, where used, is a thin parser-path adapter over that function
  - apply each boundary at most once
  - do nothing when the target placeholder is missing, so a truncated or superseded response leaves the fallback visible
  - no framing carries script, so policy:security-response-headers needs no nonce beyond the shell reference
cleanup:
  on_apply: the placeholder is replaced and the framing elements are removed, so an applied boundary leaves no id, template, or marker behind
  effect: applied boundaries cannot accumulate and cannot collide with anything later
  not_unconditional: a boundary that never settles keeps its placeholder on purpose, because that placeholder is the visible fallback
identifiers:
  scope: allocated upstream, numbered per render as tb-1, tb-2
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
