---
id: requirement:application-middleware-registration
type: requirement
title: Numbered Middleware Chain Registration
---
The whole request chain lives on one ascending number line, every framework middleware sits at a multiple of ten, and an application registers its own middleware at any number in between, so inserting a step never means rebuilding the chain by hand.

```yaml
status: implemented
audience: actor:application-developer
model: BASIC line numbers; tens are taken by the framework, the gaps belong to the application
extends: api:framework-extension, whose Slot values once ordered only the extension region; this widens that line to the whole chain
surface:
  register: pw.RegisterMiddleware(slot pw.Slot, name string, middleware pw.Middleware)
  caller: main, after every package init, exactly as pw.RegisterSessionStore and pw.RegisterConfig require
  assemble: pw.Middlewares and pw.Run compose one chain from framework frames and registered middleware, ascending, smallest outermost
  constants: one exported constant per framework frame, so an application writes pw.SlotAccessLog - 5 rather than a bare number
  mechanism: RegisterMiddleware delegates to the extension registry with a Setup that returns the middleware, so one list carries every frame and one duplicate check covers both surfaces
ordering:
  rule: ascending slot, smallest outermost; equal slots keep append order, framework frames first, then registration order
  spacing: framework frames sit at multiples of 10, so every adjacent pair has nine open positions
line:
  10: otel root span, present only when tracing exports
  20: resource injection, which everything below reads the request context through
  30: request id
  40: access log, inside request id so every line carries it
  50: recover
  60: security headers
  70: request timeout
  80: max request body
  90: public assets
  100: operational probes and framework assets, a fixed frame per api:application-lifecycle
  110: storage clients, was SlotStorage 5
  120: session resolution, was SlotSession 10
  130: authentication, was SlotAuthentication 20
  140: csrf, was SlotCSRF 25
  150: guard, was SlotGuard 30
  160: documentation endpoints, a fixed frame below the guard
fixed_points:
  outermost: the track frame stays outside the line, because its metrics must observe every registered step
  innermost: the application handler
  frames: 100 and 160 are handlers rather than middleware and refuse registration at their exact number, naming the constant to move relative to
registration_rules:
  - a duplicate name panics at registration, matching RegisterExtension
  - a nil middleware is refused rather than silently skipped
  - the chain is composed once by Run or Middlewares; a middleware registered later joins nothing, so main registers before either, exactly as RegisterSessionStore
  - a registered middleware needing resources reads them from the request context, which slot 20 guarantees for everything below it
  - pw.RegisterExtension stays the contract for an imported capability with Setup and Close; both surfaces feed one line
examples:
  request_clock:
    slot: 35, after request id and before access log
    does: captures one timestamp into a session.RequestScope slot at the top of the request
    why: a handler calling time.Now per write scatters timestamps across the request; every updated_at written by one request should carry the one moment the request began
    reads: the rdb layer and handlers load the slot instead of calling the clock
  request_id_scope:
    observation: the request id middleware already builds a per-request value and carries it in a bare context key
    direction: expose it as a session.RequestScope slot, so it is read back typed like every other per-request fact, per requirement:state-storage-tiers request_scope
migration:
  - Slot constants keep their names and change value; code using the constants recompiles unchanged
  - code that wrote a bare 5..30 now sits outside every framework frame instead of inside the extension region; every in-tree registrant uses the constants
acceptance:
  - a middleware registered at 35 observes the request id and appears in the access log's timing
  - two middlewares at one slot run in registration order
  - registration at a fixed frame number is refused naming the frame
  - pw.Middlewares returns the same chain pw.Run serves, including registered middleware
  - a registered middleware panicking is answered by the recover frame when its slot is greater than 50
non_goals:
  - reordering the track frame or the application handler
  - per-route middleware, which belongs to the mux
  - removing a framework frame by registering over it; configuration disables frames, numbers only order them
```
