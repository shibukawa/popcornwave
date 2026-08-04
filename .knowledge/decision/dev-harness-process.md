---
id: decision:dev-harness-process
type: decision
title: Development Harness Process
---
A requirement:dev-console pane that must call the project's own generated code does so through one generated pwdev main package pw builds and runs, rather than inside the pw process or inside the application process.

```yaml
status: accepted
supersedes: the storybook-only reading of this mechanism, which was the first pane to need it and not the only one
problem:
  - a generated template or query function is Go code in the project's own module, so only a process compiled from that module can call it
  - pw is a released binary that never linked the project, per decision:host-tools-target-runtime
  - the application is the one process that does link it, and policy:dev-console-boundary refuses to give it a development route
decision:
  artifact: one main package api:cli-generate emits under the pwdev build constraint
  content: blank imports of every package a pane needs, plus a server over the registries those packages register into
  build: host Go with the pwdev tag, the way api:cli-dev already builds the application
  lifetime: started with requirement:dev-console, rebuilt on the same watched changes, and stopped with the developer loop
  mount: requirement:dev-console proxies each pane to it, so the developer still sees one listener and one URL
  shared: one harness for every such pane, because a second one would double the build time of every edit for no isolation the panes need
tinygo:
  toolchain: host Go, never TinyGo, since nothing here is deployed and the harness never has to satisfy rule:tinygo-runtime-compatibility
  consequence: a pane may use a host-only engine the application could not link, which is what makes this placement worth its cost
  precedent: decision:migration-execution-split already crosses this line in the other direction, delegating host-only system:goose work from a TinyGo application out to a pw child process
  non_claim: a harness that runs is no evidence the same code links under TinyGo; api:cli-build and rule:tinygo-runtime-compatibility remain the only such evidence
consumers:
  - requirement:template-storybook
database:
  rule: a harness pane opens no database connection
  reason: requirement:contrib-sqlite is embedded, so the development connection is a process-local handle rather than an endpoint; an in-process sqlite://:memory: is unreachable outright, and a file-backed one would make the harness a second writer against a pool scaffolded at one connection
  instead: decision:dev-application-attachment serves any pane needing a live connection
  boundary: this decision therefore covers panes that need generated code and nothing running, which is why requirement:template-storybook fits it and requirement:dev-query-runner does not
alternatives_rejected:
  reinterpret_in_pw:
    shape: parse .pw.html and .pw.sql in the pw process and execute them with a host-side implementation
    why_not: it is a second implementation of flow:template-generation and flow:sql-generation, so a pane would drift from the code the application runs and would stop being evidence; it would also bypass api:instrumented-sql-executor and produce no requirement:query-diagnostics record
  serve_from_the_application:
    shape: a pwdev route in the application process
    why_not: policy:dev-console-boundary refuses a development route, and the pane would die with an application that is down most of the time between two working states
  plugin_or_shared_object:
    shape: load the project's generated code into pw at runtime
    why_not: Go plugins constrain toolchain and platform, and the release pipeline of data:release-artifact is a cross-compile that cannot promise a match
  export_everything:
    shape: make every generated symbol exported so an ordinary package could import it
    why_not: it changes the application's public surface to serve a development tool
cost:
  - one more process in the developer loop, and one more thing whose build can fail
  - a harness build failure is reported as data:dev-loop-state like any other, disables the panes that need it, and never ends the loop
relations:
  generated_output: policy:generated-artifacts governs the emitted files, which are regenerated rather than edited
  precedent: decision:passkey-test-authenticator already keeps a development-only implementation out of a package every application imports
```
