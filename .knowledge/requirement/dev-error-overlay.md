---
id: requirement:dev-error-overlay
type: requirement
title: Development Error Overlay
---
A page served by api:cli-dev shows the loop's current failure over itself, so a generation, migration, build, or startup diagnostic reaches the browser the developer is already looking at instead of only the terminal behind it.

```yaml
audience: actor:application-developer
scope: api:cli-dev only, through the reserved pwdev build mode of policy:dev-console-boundary
default: enabled
configuration: data:project-config dev.console.overlay
state: data:dev-loop-state
behavior: flow:dev-overlay-delivery
runtime_class: decision:dev-browser-runtime-scope development class, delivered by the in-application exception it describes
delivery:
  switch: a disabled overlay injects no console address, so the framework serves no development module and the core carries no import of one
  consequence: turning it off is what makes the served page byte-identical to a production render, rather than something the browser decides not to show
  isolation: the overlay is rendered into a shadow root, so the application's stylesheet cannot restyle it and it cannot restyle the application
  text: the diagnostic is written as text rather than markup, so a diagnostic quoting the developer's own HTML is read rather than run
shows:
  - the phase that failed, named as api:cli-dev names it
  - the diagnostic text unchanged, because a reformatted compiler error is harder to read than the original
  - the source location when the diagnostic carries one, linked to an editor
  - nothing at all while the loop is healthy
covers:
  - api:cli-generate failure
  - migration failure, including the down_required stop that leaves the schema alone
  - Go build failure
  - CSS build failure, which today only reaches the terminal
  - an application process that exited, whatever its status
excluded:
  - a request that failed inside a healthy application, which is an error response and belongs to api:error-renderer
  - reason: the overlay reports the state of the loop, not the outcome of a request
survives_the_application:
  rule: the overlay's stream terminates at requirement:dev-console, never at the application
  effect: a page already open keeps showing the failure while the application is down, which is when the failure matters most
  limit: a page not yet loaded cannot be served at all, so requirement:dev-console also shows the state on its index
reload:
  default: on, and disabled by dev.console.overlay.reload
  trigger: a transition to healthy whose build identity differs from the one the page was served under
  learned: the page takes its build from the first record it receives rather than from anything rendered into the document, which keeps the framework injecting nothing into a page it does not own
  window: an application replaced between the page being served and the stream connecting is not detected, which is a sub-second gap and costs a stale page rather than a wrong one
  rationale: a concept:classic-web-style page has no client state to preserve, so a full reload is the whole feature and costs nothing
  restraint: never reload while the loop is failing, because it would replace a readable diagnostic with a connection error
non_goals:
  - a debug toolbar, a query list, or a timing panel on the application page; requirement:dev-telemetry-viewer answers those in requirement:dev-console
  - preserving scroll position or form state across a reload
  - any presence in an api:cli-build artifact
acceptance:
  - a syntax error introduced mid-edit appears over the open page without the developer leaving the browser
  - the diagnostic is byte-identical to the one the terminal received
  - a fixed error clears the overlay and reloads the page once
  - the overlay stays visible while the application process is down
  - a build produced by api:cli-build serves no overlay module and no stream reference
  - dev.console.overlay false leaves every served page byte-identical to a production render
```
