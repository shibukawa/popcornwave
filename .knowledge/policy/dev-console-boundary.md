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
  effect: the application carries three development behaviors and no development route: reading assets locally, serving requirement:dev-error-overlay, and holding one outbound attachment
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
  compiled_project_code:
    runs: the project's own generated functions, through decision:dev-harness-process
    covers: what neither the tree nor a span can answer, because the answer is what the generated code does
    rule: the harness calls the generated entry point the application calls, never a reimplementation of it, so a pane keeps the api:instrumented-sql-executor seam and the requirement:query-diagnostics record that comes with it
application_side:
  admitted: the reserved pwdev build mode only
  present_uses:
    - decision:development-public-assets
    - requirement:dev-error-overlay
    - decision:dev-application-attachment
  rules:
    - a pwdev-only file carries the build constraint, so api:cli-build cannot emit it
    - api:cli-dev is the only caller that sets the tag
    - a pwdev difference changes how the application serves what it already serves, and adds no route
  refused:
    - a development route that answers with application state, such as a session list, a merged configuration dump, or a cache listing
    - reason: a route is reachable by anything that can reach the port, so one build-mode mistake turns a convenience into a disclosure
    - alternative: the same fact is reported by static analysis when it is knowable from the project, and by telemetry when it is only knowable at runtime
    - escalation: a fact reachable by neither, and addressable only from inside the process, goes through decision:dev-application-attachment, whose closed request set is what keeps it from becoming the refused route by another name
    - remainder: anything still unreachable is not served; it is recorded as a limit on the pane, the way api:cli-doctor records what it could not determine
read_only:
  default: a pane reports and does not change the project
  exception: an action a pane offers must already exist as a pw subcommand, so the console is a second way to run it and never a second implementation
  reason: api:cli-doctor refuses --fix for the same reason, and a console that edits stops being one a reader trusts
secrets:
  - a pane redacts by the rules that already bound the value, per policy:query-log-safety and policy:cookie-value-protection
  - a resolved DSN, a client secret, and a session value are masked in a pane exactly as policy:startup-summary masks them
```
