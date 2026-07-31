---
id: requirement:live-html-rendering
type: requirement
title: Live HTML Rendering
---
A page keeps updating one region after its document is complete, driven by a Go source that yields many values, with the handler and the template author writing nothing about transport.

```yaml
status:
  implemented:
    - the document response marks whether live work remains, per api:live-delivery-protocol
    - the live mode answers on the page's own route, per decision:live-delivery-transport
    - the client opens, applies, and re-establishes the connection, per requirement:live-connection-recovery
    - the bounds of policy:live-subscription-bounds, minus a per-boundary interval floor
  not_implemented:
    - suppressing an unchanged delivery server-side, which needs the boundary diffing system:tinybind has not built; the client compares instead and leaves an identical region alone
    - a per-boundary minimum interval, which is still an open question upstream about where pacing is declared
dependency: system:tinybind v0.2.7 htmlbind live boundaries
evidence:
  surveyed: 2026-07-31 against v0.2.7, tagged at 249b9c2
upstream_deferred:
  fact: htmlbind ships the language and runtime half only; boundary-level diffing, per-boundary revisions, and unchanged-delivery suppression are designed upstream and not built
  effect_here: a delivery replaces a whole boundary subtree and carries no revision, so a long list costs its length on every tick and an overlapping connection cannot be ordered against a newer one
  migration: the seam is the record, so revisions and operations replace its contents without touching a template, a source signature, or a handler
upstream_split:
  shipped_there:
    - "`external live Name(id: string): T` declarations, whose Go shape is func(ctx, args...) iter.Seq2[T, error]"
    - a live binding in an ordinary await clause; no second clause keyword, and one clause may mix a settle-once binding with a live one
    - htmlbind.RenderChainLive, which yields one htmlbind.Content per delivery and does not end
    - htmlbind.HasLiveBlock over a chain, a subset of HasAwaitBlock
    - htmlbind.Content.AppendJSON, the record form of a delivery
    - the form-control diagnostic of rule:live-boundary-authoring
    - positional boundary ids, which repeat across executions of the same chain
  left_here: every question about the wire, the connection, and the client, because htmlbind writes no headers, chooses no framing, and ships no script
  reason: the same ownership line api:html-boundary-protocol already draws for settled boundaries
handler_delta:
  required: nothing beyond declaring the source and implementing the Go function; a live source is called by generated code, never by the handler
  unchanged:
    - api:html-response call site and signature, per decision:automatic-async-render-selection
    - api:async-html-value, which stays the settle-once path
    - decision:implicit-document-shell document resolution
template_delta:
  - declare `external live` for a source that keeps producing
  - bind it in an await clause; fallback is required and recover is optional, exactly as for a settle-once binding
  - a delivery carries the whole state of the region, not an increment, so a chat source yields the current list
  - rule:live-boundary-authoring governs what the primary subtree may contain
document_response:
  branch: the existing streaming branch of decision:automatic-async-render-selection, unchanged
  behavior: a live boundary commits its fallback, takes its first delivery as an ordinary completion when one arrives inside html.async_timeout, and then unsubscribes, so the document still ends
  quiet_source: running out of time leaves the committed fallback and reports nothing, because a source with nothing to say yet has not failed; this differs from an await binding, whose timeout stays a failure
  bot_and_buffered: the buffered branch renders the first delivery in place and stops watching, so requirement:bot-synchronous-render needs no live-aware path
continuation:
  who: api:live-delivery-protocol carries deliveries after the document, on the transport decision:live-delivery-transport chooses
  when: only when htmlbind.HasLiveBlock reports true for the chain, so a screen that will never change again costs no speculative request
  recovery: requirement:live-connection-recovery owns reconnect, and the client half lands in requirement:external-boundary-runtime
bounds: policy:live-subscription-bounds
configuration: data:html-render-config
flow: flow:live-boundary-delivery
criteria:
  - a chain with no live binding renders byte-identically to today, on both branches
  - a source yielding every few seconds re-renders its region on that cadence with no polling from the client
  - a delivery re-renders only its own boundary subtree; siblings and ancestors are untouched
  - a client that runs no script keeps the content the document committed, which is a real first delivery rather than a permanent placeholder
  - a classified bot receives a settled document with no live framing in it
  - a disconnect leaves no source goroutine running
  - a live source failure renders recover and a later value replaces it with primary content
  - a live boundary in a chain that also has settle-once boundaries does not delay the document past what those boundaries already cost
non_goals:
  - a browser-to-server channel; a live boundary is one-directional and a client-initiated change stays flow:partial-refresh
  - a second route per page
  - pub/sub, fan-out, or presence; sharing one upstream across clients is the application's job inside the source
  - replacing api:typed-stream, which is typed JSON event delivery for API handlers rather than HTML boundary rendering
upstream_asks:
  raised: 2026-07-31 against the shipped runtime, and all three accepted upstream, which verified each against its own source before agreeing
  order_there: the lock first because it was nearly free and the only defect, liveness second because it is additive, the slice last because it needs a design round of its own
  reporting: requirement:live-error-report-off-lock, done in v0.2.8; nothing here changed but a stated constraint went away
  liveness: requirement:live-boundary-liveness-signal, accepted and not yet built; paid for here in DOM footprint and in a full retransfer per rotation
  slicing: requirement:live-mode-plan-slice, accepted and not yet built; it is the one cost no dial here can relocate, and the ranking item for both sides
example:
  project: examples/live_render
  page: one timer-paced gauge, one event-paced room fed by a source many readers share, one clause mixing a settle-once binding with a live one, and one static panel that proves a reconnect repaints nothing else
  configured_short: a 30s lifetime and a 20s idle bound, so a rollover is something a reader can watch rather than wait out
  verified_in_a_browser: five successive connections across jittered lifetimes, a server restart recovered by backoff with no reload, and the static panel untouched throughout
open_questions:
  - whether a live boundary is worth a diagnostic when it renders a list, which is the shape most likely to want role log
```
