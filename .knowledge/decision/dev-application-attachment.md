---
id: decision:dev-application-attachment
type: decision
title: Development Application Attachment
---
A requirement:dev-console pane needing the application's own live database connection is served by the application dialing out to the console under pwdev and answering a closed set of requests, never by the console dialing in.

```yaml
status: accepted
problem:
  connection_is_not_addressable:
    - requirement:contrib-sqlite is an embedded engine, so a connection is a process-local handle and not an endpoint anything can reach
    - an in-process sqlite://:memory: database has no external existence at all, which decision:migration-execution-split already records as a hard constraint on delegation
    - decision:test-migration-snapshot already rejected the workarounds: file::memory:?cache=shared is shared within one process only, and a file copy cannot populate an already-open in-memory database
  file_backed_is_reachable_but_contended:
    - the api:cli-init SQLite scaffold sizes the pool at one connection because SQLite serializes writers on one file
    - a second process opening the same file is a second writer, so a pane could block the application it is meant to observe
  consequence: decision:dev-harness-process resolves its own DSN, which answers for a server engine and fails for the default one
decision:
  direction: the application connects to requirement:dev-console; the console never opens a connection to the application
  selection: the reserved pwdev build mode, per policy:dev-console-boundary
  address: the console URL api:cli-dev already injects into the application process
  credential: a token generated per run from crypto/rand and injected the way requirement:contrib-devidp injects its client secret, so an attachment is not accepted from anything that merely reached the port
  transport: one loopback connection the application holds open, re-established on the console's terms after a restart
why_not_an_inbound_route:
  objection: policy:dev-console-boundary refuses a development route because a route is reachable by anything that can reach the application port
  resolution: an outbound attachment is reachable only by the address the application was given, so the objection is answered rather than waived
  precedent: the flow:telemetry-export exporter already dials out to requirement:dev-telemetry-viewer, so the direction is the established one
closed_set:
  rule: the attachment answers an enumerated set of requests, never a general remote call surface
  today: run one declared requirement:dev-query-runner statement with typed parameters and return its typed result
  refused_here_as_elsewhere: arbitrary SQL, a merged configuration dump, a session listing, a cache listing
  reason: an open set is the development route this decision exists to avoid, arrived at by a different path
gains:
  - the pane runs against the connection the application already holds, so an in-memory database is reachable and a file-backed one takes no second writer
  - a run goes through api:instrumented-sql-executor natively, so its data:query-record is the application's own rather than a harness imitation of one
  - data:query-diagnostics-config, the transaction depth, and the pool are the ones the project configured
costs:
  availability:
    effect: the pane is unavailable while the application is down, which is much of the time between two working states
    accepted: inherent rather than chosen; a database whose only handle has exited cannot be queried by anything
    reporting: the pane says the application is detached, and requirement:dev-console shows the data:dev-loop-state that explains why
  tinygo:
    rule: the attachment introduces no host-only dependency, so a future api:cli-dev that builds with TinyGo is not foreclosed
    note: today the loop builds with host Go and the tag, so this is a constraint kept on purpose rather than one currently enforced by the build
relations:
  harness: decision:dev-harness-process keeps every pane that needs generated code but no live connection, which is requirement:template-storybook
  boundary: policy:dev-console-boundary places this tier
```
