---
id: decision:dev-application-attachment
type: decision
title: Development Application Attachment
---
The application serves requirement:dev-data-pane itself on a loopback listener of its own and announces that address to requirement:dev-console, which proxies to it; the console never opens a connection to the application.

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
  serves: the application, under the reserved pwdev build mode of policy:dev-console-boundary
  listener: a loopback listener of its own, never the application's own listener
  announcement:
    direction: outbound, from the application to the console at startup
    carries: one address per request and nothing else, so there is no shape for it to carry anything the console must then decide about
    credential: a token generated per run from crypto/rand and injected the way requirement:contrib-devidp injects its client secret
    refused: an announcement without the token, so an address is not taken from anything that merely reached the port
    kinds:
      pane: the loopback address the console proxies the pane to
      listening: the URL the application bound, which decision:development-port-shift can move off the configured port and which no file the console reads can be trusted for; refused unless it is an http URL, since the console renders it as a link
  reaching: requirement:dev-console proxies the pane, so the developer sees one URL and the application's dev listener is never an address anyone types
revision:
  was: the application dialling out and answering over that connection, with an enumerated closed set of requests
  why_changed:
    mechanism: a reverse tunnel is machinery, and a second loopback listener reaches the same place, since what the objection was ever about is the application's own listener
    closed_set: requirement:dev-query-runner accepts a statement, so a request no longer names a selection and an enumerated set no longer describes anything
  what_survives: the direction. The console still never dials in, so nothing has to be reachable from outside for the pane to work
bounds:
  what_is_no_longer_one:
    - the shape of a request, since SQL is accepted
    - the data reachable, since a statement can read any table the connection can
  what_still_is:
    build_mode: the pwdev constraint, so an api:cli-build artifact contains no listener, no handler, and no announcement; this is structural rather than a check
    listener: loopback for both the console and the application's dev listener
    route: nothing on the application's own listener, so the port it serves gains no surface
    token: per run, generated, never written into the project, discarded at shutdown
    database: whatever the running application opened, which api:cli-dev started against the development environment
  honest_summary: the pane is bounded by never existing outside a development loop, not by what it declines to do inside one
gains:
  - the pane runs against the connection the application already holds, so an in-memory database is reachable and a file-backed one takes no second writer
  - a run goes through api:instrumented-sql-executor natively, so its data:query-record is the application's own
  - data:query-diagnostics-config, the pool, and the session settings are the ones the project configured
costs:
  availability:
    effect: the pane is unavailable while the application is down, which is much of the time between two working states
    accepted: inherent rather than chosen; a database whose only handle has exited cannot be queried by anything
    reporting: the pane says the application is not attached, and requirement:dev-console shows the data:dev-loop-state that explains why
  tinygo:
    rule: the attachment introduces no host-only dependency, so a future api:cli-dev that builds with TinyGo is not foreclosed
    note: today the loop builds with host Go and the tag, so this is a constraint kept on purpose rather than one the build enforces
relations:
  harness: decision:dev-harness-process keeps every pane that needs generated code but no live connection, which is requirement:template-storybook
  consumers: requirement:dev-data-pane and requirement:dev-query-runner, which share one attachment and one pane
  boundary: policy:dev-console-boundary places this tier
```
