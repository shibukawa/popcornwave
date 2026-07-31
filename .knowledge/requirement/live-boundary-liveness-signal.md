---
id: requirement:live-boundary-liveness-signal
type: requirement
title: Liveness Must Be Visible Per Boundary
---
The wire must say which boundaries keep delivering, because the framework decides what to keep an address for and what to transfer on a reconnect, and today it cannot tell a live boundary from a settle-once one.

```yaml
owner: system:tinybind
status: accepted upstream 2026-07-31 and not yet built, sequenced after the reporter fix v0.2.8 shipped and before requirement:live-mode-plan-slice; requirement:live-html-rendering ships without it
priority: should
what_is_missing:
  placeholder: a live boundary and an await boundary emit byte-identical markup, `<tb-boundary id="tb-N" style="display:contents">`
  delivery: htmlbind.Content carries BoundaryID and HTML for both kinds
  chain_level_only: htmlbind.HasLiveBlock answers for a whole chain, which is the right question for whether to invite a connection and the wrong one for what to do with a given boundary
  verified: v0.2.7 htmlbind/ops.go writes the same placeholder from awaitOp and liveOp, and htmlbind/async.go declares Content with two fields
when_it_bites:
  every_streamed_document: the client must keep a re-addressable position for every boundary it applies, because any of them might be re-rendered later
  every_reconnect: a live-mode execution re-renders the page's settle-once boundaries too, and they are yielded as ordinary deliveries the framework cannot filter
  every_bound: policy:live-subscription-bounds counts boundaries per response, so html.live_max_boundaries bounds live and settled boundaries together rather than the subscriptions it is meant to bound
  not_when: a page with no live boundary at all, which never opens a connection
what_it_costs_here:
  dom: two comment nodes per applied boundary, kept for the life of the document
  runtime_code: 43 lines of the 341-line client runtime — bracket, refill, and the stale-range branch of applyBoundary — where a settle-once-only client needs 3
  bytes_per_reconnect: the rendered HTML of every settled boundary on the page, on every reconnect, which at the default lifetime is once per client every ten minutes
  suppression_is_client_side: the client compares an arriving delivery against what that region already shows and leaves identical nodes alone, so nothing repaints; the transfer still happened
  not_a_correctness_cost: a settled boundary whose data changed since the document render is legitimately worth applying, so the wasted case is the unchanged one rather than all of them
workaround_difficulty:
  possible: yes, and it is what shipped
  why_an_address_is_needed_at_all: a delivery replaces a region that is no longer bracketed by anything after the first apply, since the placeholder element is consumed
  why_not_node_references: an empty render — a list boundary whose source yields nothing — leaves no nodes, so a position derived from the content disappears exactly when a later delivery needs it; the address has to be independent of what it holds
  why_not_learn_it: liveness could be inferred from a second delivery for one id, but the address must already exist when that delivery arrives, so learning is always one delivery too late
  why_not_keep_the_element: keeping `<tb-boundary>` in the document is the other option and costs an element per boundary instead of two comments, which affects CSS and layout where comments do not
  residual: the DOM footprint and the reconnect bytes cannot be removed from the client side at all; only the module knows which boundaries are live
proposed_shape:
  either: a flag on the placeholder, so the client knows at parse time which regions need an address
  or: a field on Content, so the framework knows per delivery and can drop a settled boundary's delivery on a live response
  preferably_both: the first decides what the client keeps, the second decides what the server transfers, and they answer different questions
  compatible: an added attribute and an added field are additive; a caller ignoring both behaves exactly as this one does today
acceptance:
  - a client can decide, before any delivery arrives, which boundaries need a re-addressable position
  - a live-mode response can omit a settle-once boundary's delivery without comparing content
  - a per-response boundary bound counts subscriptions rather than boundaries
  - a chain with no live boundary produces byte-identical output to v0.2.7
```
