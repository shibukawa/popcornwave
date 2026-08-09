---
id: decision:transport-compatibility-fallback
type: decision
title: Superseded — There Is No Compatibility Fallback
---
This framework proposed running a handler the transform could not take through a net/http compatibility layer, slower but unchanged; system:tinybind shipped a refusal contract instead, so a handler that cannot be rewritten fails the build.

```yaml
status: superseded 2026-08-09 by the upstream decision, tinybind-go v0.4.9
id_kept: the identifier is not reused or renumbered, the convention data:diagnostic-check states, so the concepts referring to it still resolve and the reversal is visible where it was decided
what_was_proposed_here:
  claim: a handler naming net/http types would be wrapped by an adapter, costing allocations and preserving behavior
  consequence_claimed: the containment rule was a budget rather than a gate, and rule:transport-handle-checks reported at warning severity
what_shipped_instead:
  contract: a function the rewriter cannot take is a generation error naming the declaration; the upstream adapter boundary was specified and not implemented
  granularity: admission closes over the same-package call graph, so a refused shared helper refuses every handler calling it
  reason_upstream_gave: without a fallback one refused helper makes the application unbuildable rather than one route slower, which is what forced the transform to be transitive rather than per handler
  adoption: all-or-nothing per build; a service cannot move route by route, one binary serves one transport, and net/http-shaped third-party middleware is unusable in that build
why_the_reversal_is_better_than_it_looks:
  the_fallback_was_worse_than_it_sounded: a buffering adapter preserves neither streaming nor a raw connection, so the guarantee it offered was already holed exactly where this framework needs it
  a_refusal_is_actionable: the upstream diagnostics requirement makes the message name the occurrence, the chain from the handler that inherited it, and a remedy, which a silent slow path never would
  the_cost_lands_before_deployment: a build error is found by whoever ran the build, where an adapter's cost is found by whoever reads a flame graph months later
consequences_here:
  - decision:transport-handle-containment is a gate, not a budget
  - rule:transport-handle-checks reports what would fail a fasthttp build rather than what would merely slow it
  - requirement:pw-call-registration becomes load-bearing: an unregistered pw call is an error this framework's users cannot fix themselves
  - a report-only mode exists upstream, so a project can see the whole cost before committing rather than after
```
