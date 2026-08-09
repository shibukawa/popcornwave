---
id: decision:shared-check-catalog
type: decision
title: Shared Check Catalog
---
Startup validation and api:cli-doctor read one internal catalog of data:diagnostic-check definitions, because the same condition implemented twice diverges.

```yaml
status: proposed
problem:
  - api:application-lifecycle must reject a fatal misconfiguration before request acceptance
  - requirement:project-diagnostics must report every condition, fatal or not, without starting anything
  - the overlap is large, and a second implementation of "session backend is not linked" drifts in wording, severity, and edge cases
decision:
  ownership: one internal package holds every check definition and its message
  selection: a runner picks checks by declared inputs and phase, never by copying logic
  startup:
    runs: checks whose inputs exist in-process and whose severity is fatal for the resolved environment
    behavior: unchanged; the first fatal finding still fails before request acceptance
    warnings: emitted as warnings where policy already says so, as policy:query-log-safety does, and never promoted to failures
  doctor:
    runs: every check whose inputs it can build from the project
    behavior: collect all, rank, and report
  vet:
    runs: checks declaring typed application Go syntax as an input, which is rule:transport-handle-checks today
    form: a go/analysis analyzer, so the runner is go vet or a linter rather than this framework's CLI
    behavior: report positions in application source; startup and doctor skip these checks for lack of the input
input_asymmetry:
  startup_only: the deployment's environment variables, CLI arguments, live connections, and the actual registrations
  doctor_only: project files, the import graph, data:route-table, generated artifacts, other environments' configuration files
  vet_only: the typed syntax of application Go packages, which decision:host-side-diagnostic-analysis keeps out of doctor
  shared: the merged configuration values the check reads
  consequence: a check is not expected to run everywhere; its declared inputs decide, and each runner reports what it skipped
rules:
  - one condition has one identifier, one title, and one remedy, wherever it fires
  - a check never reads a source outside its declared inputs, so a startup check cannot start depending on the filesystem
  - adding a check is adding a catalog entry, and its documentation page follows from data:diagnostic-check
  - the catalog is internal; applications and plugins do not register checks in the first release
consequences:
  - a message an operator saw at startup is the message doctor prints, which is what makes the identifier worth citing
  - the catalog is the source for the generated diagnostics documentation
  - a check whose inputs only doctor has is normal rather than an exception, which keeps startup from growing filesystem knowledge
```
