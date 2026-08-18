---
id: decision:slot-lifetime-axis
type: decision
title: Slot Lifetime as an Independent Axis
---
How long a registered slot lives is declared beside what the client may do with it, because the two are independent: a value may always die before its session, and a value the browser holds may outlive it.

```yaml
status: accepted
state: implemented, per api:session-registry
axes:
  trust: what the client may do with the value
  placement: where the bytes live
  lifetime: what ends the value, which this decision adds
  independence: the third is free of the first two, with one constraint below
constraint:
  rule: a slot may always state a shorter life; only a cookie-placed slot may outlive the session
  reason: a record-placed value is destroyed with the record that holds it, so outliving is not a policy this framework declines to offer but a thing that cannot happen
  enforcement: registration, not startup, because the placement is known at the declaration
surface:
  session.ExpiresAfter: bounds the slot to a duration whatever its placement
  session.OutlivesSession: keeps the slot for a duration and exempts it from the destruction of the session
  session.BrowserMax: the longest a browser will keep a cookie, which is what "indefinitely" can mean
  default: stating nothing ties the slot to the session, which is the conservative answer and the previous behavior
combinations:
  ends_with_session_and_shorter: a rotating secret, a step-up admission, a cached decision
  outlives_and_bounded: a density preference kept for a season
  outlives_and_browser_max: a display language, which is the case that motivated the axis
  ends_with_session_and_browser_max: writable but unnamed, because ExpiresAfter(BrowserMax) already says it
record_slots:
  mechanism: a fixed-width deadline prefixed to the slot's own encoded value inside data:session-record
  scope: per slot, so one short-lived slot expires while the session and its other slots continue
  read: a slot past its deadline reads as absent, exactly as one never written
  evidence: popcornweb/plugin/auth already hand-rolled this, storing a unix stamp in its payload and comparing it against a thirty-second window; the primitive was wanted before it existed
cookie_slots:
  mechanism: the stated duration becomes the cookie lifetime
  default: the session lifetime, which is what makes a slot that stated nothing die with the session rather than at the next browser close
  guarantee_differs_by_tier:
    shared: plain carries no expiry stamp, so the duration is the Max-Age attribute alone and a client that rewrites the value discards it
    read_only: signed carries the stamp inside the authenticated payload, so an expired value presented later is refused
    consequence: the same argument buys a weaker promise on session.Shared, which is the same reason policy:cookie-value-protection gives for a plain value carrying no expiry
browser_max:
  fact: HTTP has no "never expires"; a cookie with no Max-Age dies at browser close and one with a Max-Age is capped
  value: 400 days, the current browser cap
  separate_from_session_ttl: the session lifetime keeps its own year-long bound, because a cookie retention limit and a session lifetime limit are different questions
rejected_alternatives:
  - enumerating the valid combinations as distinct placement values, which reaches eight names and grows by multiplication; the deployment interaction of session.ServerOnly with the cookie backend cannot be made unrepresentable anyway, so a second kind of check would leave a reader learning which errors are compile-time and which are startup
  - collapsing the axis into one option whose presence implies the scope, which was refused because a value shorter-lived than its session is an ordinary want rather than a degenerate cell
  - a cookie-placed default of no Max-Age, which would kill a session-scoped slot at the next browser close while the session it belongs to lives on
```
