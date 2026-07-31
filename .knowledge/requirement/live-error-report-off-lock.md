---
id: requirement:live-error-report-off-lock
type: requirement
title: Report A Live Failure Without Holding The Boundary
---
A live boundary must not hold its lock while calling the render error reporter, because the reporter is the caller's logger and a subscription that stalls behind a log write stops updating a screen nobody can see is stale.

```yaml
owner: system:tinybind
status: implemented upstream in v0.2.8, taken first of the three live integration requests because it was nearly free and the only one that was a defect rather than a missing capability
shipped:
  shape: the render callback returns the failure to report instead of reporting in place, and the pump hands it to the reporter after releasing the delivery lock
  scope: the synchronous entry runs the same pump and had the same problem, so both paths are covered
  no_signature_change: the caller's reporter is unchanged, and no caller can observe the difference except by not stalling
priority: should
when_it_bites:
  path: the live entry only; an await boundary already reports off the lock
  trigger: a live source yielding an error, a delivery whose render fails, or a recovered panic in a source
  amplifier: a logger that blocks — a full pipe, a slow stdout, a synchronous exporter, a rotating file handler
  correlated: the case that produces a burst of failure deliveries is usually the same outage that slows the log pipeline, so the two arrive together rather than independently
  consequence: the boundary cannot take another delivery until the log write returns, so the region freezes while the rest of the page keeps updating
  worse_than_a_delay:
    correction: an earlier reading here called this freshness only, which was wrong
    fact: the reporter takes no context, so a reporter that blocks indefinitely holds the clause's goroutines and sources rather than only delaying them, and cancellation does not free them
    bounded_by: the response itself still ends, because the consumer returns on its own context
what_it_costs_here:
  today: nothing measured; api:logger writes are fast on a healthy host, which is what makes this a latent fault rather than a visible one
  exposure: policy:live-subscription-bounds requires that a reporter passed by api:html-response must not block, which is a constraint the framework states and cannot enforce
workaround_difficulty:
  possible: yes, and rejected
  shape: hand each report to a buffered channel or a goroutine, so the reporter returns immediately
  cost: roughly 40 lines plus tests, a dropped-record counter, and a queue depth to choose
  why_rejected:
    - it trades an error record for freshness, which is the wrong way round: an operator loses the log line describing the outage precisely during the outage
    - it puts a log queue inside the response path, which policy:log-emission keeps out of the framework deliberately
    - it reorders records against every other log this process writes, so a live failure would no longer sit between the requests around it
  narrower_alternative: a reporter that drops only when it would block, which is the same trade in a smaller package
downstream_effect:
  code: none; api:html-response passes an api:logger write, which was already the reporter this needed
  constraint_lifted: policy:live-subscription-bounds no longer has to require a non-blocking reporter, which was a rule the framework stated and could not enforce
acceptance:
  - a reporter that blocks for a second delays no delivery of any boundary
  - a failure delivery is still reported exactly once, in the order it occurred for that boundary
  - the await path's behaviour is unchanged
```
