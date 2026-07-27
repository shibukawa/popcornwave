---
id: requirement:unhandled-boundary-signal
type: requirement
title: Unhandled Boundary Signal
---
system:tinybind surfaces an await boundary that failed with no recover clause, so omitting recover delegates the failure to the caller instead of silently choosing an endless fallback.

```yaml
owner: system:tinybind htmlbind
delivered: v0.1.21
surface:
  type: "htmlbind.UnrecoveredError with BoundaryID and Err, unwrapping to the cause so ordinary error mapping still reaches it"
  async_entry: yielded through the render sequence, which then ends
  blocking_entry: returned instead of rendering the fallback subtree
  boundary_id: empty on the blocking path, which writes no placeholder
reframing:
  today: omitting recover means the fallback stays, decided by the module
  wanted: omitting recover means the caller decides, which for a framework is decision:unhandled-boundary-escalation
  why: an author who wrote no recover clause said nothing about failure, and a permanent loading state is not a plausible reading of nothing
former_behavior:
  async_entry: the boundary returned a not-present result and the coordinator dropped it, so the sequence never mentioned it
  blocking_entry: the fallback subtree was rendered in place, producing a finished document showing a loading state that could never resolve
  gap: WithErrorReporter fired but carried no boundary identifier and did not say whether a recover clause absorbed the failure
rationale:
  terminal_async: ending the range is correct, because the caller is about to replace the page rather than patch it
  caller_owns_the_loop: an in-render callback would have had to signal that loop anyway
  migration_softness: a caller written against the documented loop already breaks on a yielded error, so it degrades to stopping and logging rather than to a stuck fallback
  blocking_payoff: nothing is committed on that path, so a caller holding a buffer discards it and answers with a real error status
stays_silent:
  cancellation: a cancelled request or an early consumer stop must not escalate, and already returns before the reporter is called
  reason: nobody is reading the response, so there is nothing to escalate to
still_escalates:
  timeout: a boundary that ran out of time is a failure like any other; recover is how an author keeps one contained
not_needed_upstream:
  status_mapping: the original error reaches the caller, so a framework maps it with its own rules
  public_projection: PublicError stays the recover clause's concern, since an escalated page is rendered by the framework rather than by the template
recommended_documentation:
  audience: the framework owner guide
  content:
    - omitting recover delegates rather than silences
    - the framing and client runtime that apply a replacement belong to the caller, as api:html-boundary-protocol already records
    - a document replacement is parser-driven too, so it needs the same trailing marker discipline as a completion
    - a streaming escalation cannot change a committed status, and a buffered one can
compatibility:
  kind: deliberate behavior change, not an addition
  affected: a direct htmlbind user ranging without handling the new signal now stops early
  alternative: gate it behind an option, at the cost of a setting whose right value is always on for a framework
non_goals:
  - deciding what replaces the page, which belongs to the framework
  - making recover mandatory at generation time, which would turn a framework default into per-boundary boilerplate
```
