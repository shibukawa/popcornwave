---
id: policy:dev-console-boundary
type: policy
title: Development Console Boundary
---
A requirement:dev-console pane is built from what the host can already read, receive, or compile for itself, and the only development behavior admitted into the application is the reserved pwdev build mode.

```yaml
placement:
  question: a pane needing to run project code could live in the application, which already links it; this names why it does not
  not_the_reason:
    claim: TinyGo compatibility forbids development code in the application
    fact: api:cli-dev builds the application with host Go and the pwdev tag, so nothing in the developer loop meets TinyGo at all
    where_it_does_bite: what api:cli-build ships, and what api:test-run compiles under a TinyGo test binary; a development pane reaches neither
  the_reasons:
    availability: the application is stopped for much of the time between two working states, and a tool that reports on a broken build must outlive it
    surface: a development route is reachable by anything that can reach the port, so one build-mode mistake turns a convenience into a disclosure
    linkage: a pane may want a host-only engine, and putting it in the application would either fail to link under TinyGo or force the pane down to what TinyGo carries
  order:
    - the pw process, when the project tree or a pw-owned resource already answers
    - decision:dev-harness-process, when only the project's generated code answers and no live connection is involved
    - decision:dev-application-attachment, when the answer requires a resource only the running application can address, which requirement:contrib-sqlite makes the ordinary case rather than the exotic one
    - the application under pwdev, only when the code must run inside a page the application served
  effect: the application carries three development behaviors and no development route on its own listener: reading assets locally, serving requirement:dev-error-overlay, and serving requirement:dev-data-pane on a loopback listener of its own that it announces to the console
information_sources:
  static_analysis:
    reads: the project tree, its Go packages, and its generated artifacts
    owner: decision:host-side-diagnostic-analysis, whose rules api:cli-doctor already follows
    rule: never treat an undeterminable value as its default
  telemetry:
    reads: what the application exported to requirement:dev-telemetry-viewer
    covers: requests, spans, api:logger records, and requirement:query-diagnostics records
    rule: a pane that wants runtime facts asks for an instrument here rather than an endpoint in the application
  host_owned_resources:
    reads: what pw itself started or connected, such as requirement:contrib-devidp and the data:migration-source state
    rule: pw resolves the connection from project configuration; it does not borrow the application's
  the_running_application:
    serves: what only the process holding the connection can reach, through decision:dev-application-attachment
    covers: requirement:dev-data-pane and requirement:dev-query-runner
    bound: not by what the pane declines to do, but by never existing outside a development loop; the pwdev constraint is what makes that structural
  compiled_project_code:
    runs: the project's own generated functions, through decision:dev-harness-process
    covers: what neither the tree nor a span can answer, because the answer is what the generated code does
    rule: the harness calls the generated entry point the application calls, never a reimplementation of it, so a pane keeps the api:instrumented-sql-executor seam and the requirement:query-diagnostics record that comes with it
application_side:
  admitted: the reserved pwdev build mode only
  present_uses:
    - decision:development-public-assets
    - requirement:dev-error-overlay
    - requirement:dev-console-launcher, admitted by decision:dev-launcher-admission because it navigates rather than answers
    - decision:dev-application-attachment
  rules:
    - a pwdev-only file carries the build constraint, so api:cli-build cannot emit it
    - api:cli-dev is the only caller that sets the tag
    - a pwdev difference changes how the application serves what it already serves, and adds no route
  refused:
    - a development route that answers with application state, such as a session list, a merged configuration dump, or a cache listing
    - configuration_answered_elsewhere: the question a dump is asked for is answered by the api:cli-doctor pane, which merges the same layers, masks what policy:startup-summary masks, and names a remedy for what it finds
    - reason: a route is reachable by anything that can reach the port, so one build-mode mistake turns a convenience into a disclosure
    - alternative: the same fact is reported by static analysis when it is knowable from the project, and by telemetry when it is only knowable at runtime
    - escalation: a fact reachable by neither, and addressable only from inside the process, goes through decision:dev-application-attachment, which serves it on a loopback listener of its own rather than on the application's, so the refused route stays refused
    - remainder: anything still unreachable is not served; it is recorded as a limit on the pane, the way api:cli-doctor records what it could not determine
changing_things:
  project: no pane edits the project. Source, configuration, and generated output are read, because api:cli-doctor refuses --fix for the reason that a diagnosis which edits stops being one a reader trusts
  running_state: a pane may change what the running application is working on, which today means the rows in its development database
  why_the_difference:
    project: it is the developer's work, versioned, and every way to change it already exists as an editor or a pw subcommand
    development_data: it is neither versioned nor durable — api:cli-seed refills it and a migration cycle empties it — and being unable to fix a row is what sends the developer to the second tool decision:dev-console-self-sufficiency exists to avoid
  bound: requirement:dev-data-pane and requirement:dev-query-runner reach only the database the running application opened, which api:cli-dev started against the development environment
secrets:
  - a pane redacts by the rules that already bound the value, per policy:query-log-safety and policy:cookie-value-protection
  - a resolved DSN, a client secret, and a session value are masked in a pane exactly as policy:startup-summary masks them
```
