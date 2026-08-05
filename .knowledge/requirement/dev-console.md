---
id: requirement:dev-console
type: requirement
title: Development Console
---
api:cli-dev serves one loopback web console whose panes answer what the project is, what it would serve, and what it just did, so the developer loop has a browser surface beside its terminal stream.

```yaml
audience: actor:application-developer
mechanism: decision:dev-console-consolidation
hosting: decision:dev-console-self-sufficiency decides what the console answers itself and what it leaves to a tool the developer already has
boundary: policy:dev-console-boundary
delivery: decision:dev-console-delivery-order
scope: api:cli-dev only; never api:cli-build, api:test-run, or any deployed environment
default: enabled
configuration: data:project-config dev.console
listener:
  bind: loopback only
  port: dev.console.port, a fixed default rather than a reserved one, because a console is bookmarked and returned to; the precedent is the dev.idp.port pin
  collision: a bound port fails the console, reports the address, and leaves the developer loop running
  mount: index plus every pane on one listener, so one printed URL reaches all of them
  url: printed once at startup beside the requirement:contrib-devidp report
panes:
  telemetry:
    concept: requirement:dev-telemetry-viewer
    scope: traces, logs, and per-request timing; the console adds no log pane and no profiler of its own
  storybook: requirement:template-storybook
  data:
    browsing: requirement:dev-data-pane
    running: requirement:dev-query-runner
    shared: one pane and one decision:dev-application-attachment; the console proxies it because the application serves it
  assets: requirement:dev-asset-inspector
  api: requirement:dev-api-reference
  loop_state:
    concept: requirement:dev-error-overlay
    surface: the index shows the current data:dev-loop-state, so a page that was never opened still reports the failure
index:
  content: the project name, the diagnosed environment, the application URL, and one entry per enabled pane
  reseed:
    offered: when the project has seed datasets and seed.auto is on
    action: api:cli-seed, the same call the loop makes after a migration cycle empties the database, rather than a second implementation
    why_here: seeding is clear-insert, so it is the way back from an editing session in requirement:dev-data-pane, and it is a pw-side action that needs no attachment
    said_plainly: the button names what clear-insert does, because a control that empties tables should not read as a refresh
  disabled_pane: named as disabled with the configuration key that enables it, rather than hidden
shell:
  form: one document per pane, navigated to by ordinary links from a nav the index and every pane repeat
  not_a_single_document: one listener and one index do not imply one page; decision:dev-console-consolidation joined the listener, not the document
  rationale:
    third_party_isolation: an embedded renderer such as requirement:dev-api-reference ships global CSS written to own a document, and giving it one costs less than isolating it inside a shared shell
    independent_failure: a pane whose bundle is broken breaks its own page and leaves the rest of the console usable
    per_page_policy: a permissive third-party bundle takes a permissive response policy on its page alone, instead of lowering the whole console to its level
    dogfood: concept:classic-web-style is what pw tells applications to do, and a console built any other way argues against its own framework
  iframe:
    used: no
    why_not: same-listener panes are same-origin pages already, so an iframe would add sizing, scrolling, and theme coordination to buy isolation a separate document gives for free
    when_it_would_be: only to keep two panes visible at once or to preserve a pane's state while another is open, and neither is a goal
  spa_panes: a pane whose renderer is a browser application is still one page; requirement:dev-telemetry-viewer is that case and needs no exception
lifetime:
  start: before the application process, like requirement:contrib-devidp
  restart: the console keeps its listener, its port, and everything captured across regeneration, migration, rebuild, and restart
  stop: with the developer loop
  failure: report diagnostics and keep the loop alive, because an unobservable run is still a working one
injection:
  variable: the resolved console URL, on the application process only, by the mechanism requirement:dev-telemetry-viewer already uses
  consumer: requirement:dev-error-overlay, which needs an address the served page can reach
  rule: never exported to the developer shell
harness:
  concept: decision:dev-harness-process
  needed_by: requirement:template-storybook, which runs the project's own generated code and reaches no database
  effect: the pane is proxied rather than served, and its availability depends on a build succeeding
attachment:
  concept: decision:dev-application-attachment
  needed_by: requirement:dev-data-pane and requirement:dev-query-runner, whose connection only the application can address
  effect: the pane is available exactly while the application is up, which is the one pane for which that is true
packaging:
  ui:
    preference: take a third-party UI as a Go package from its own project, as requirement:dev-telemetry-viewer takes viewer/webui, rather than committing a build of it here
    reason: a committed copy is refreshed by hand and drifts, and the divergence it hides is usually something the dependency can fix once for every embedder
    constraint: however it arrives, it arrives as Go source, so data:release-artifact stays a pure-Go cross-compile with no Node toolchain
  boundary: host-only tooling under decision:host-tools-target-runtime, never linked into an application binary
  attribution: a third-party build committed here would carry its upstream notice and SPDX expression; one linked as a package carries its own
security:
  - loopback binding only
  - no credential is held and no application route is served
  - cross-origin reads from the application origin are allowed for loopback origins only, because the application and the console are different ports
acceptance:
  - pw dev with no console configuration serves the index on the configured loopback port and prints its URL
  - the index names every enabled pane and every disabled one
  - an application restart preserves what the console captured before it
  - dev.console.enabled false starts no listener and reserves no port
  - a console that cannot listen reports the failure and leaves the loop running
  - a binary produced by api:cli-build contains no console code
non_goals:
  - sending application requests from the console, because a browser and an HTTP client already do it
  - a log pane or a request profiler pane separate from requirement:dev-telemetry-viewer
  - authentication on the console, which loopback binding already bounds
  - persistence across pw dev runs
  - opening a browser on restart; a single explicit open at startup is the most that is considered
```
