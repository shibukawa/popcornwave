---
id: rule:transport-handle-checks
type: rule
title: Transport Handle Checks
---
The PW06xx data:diagnostic-check entries: how the upstream transform's refusals reach a developer here, and what this framework reports that the upstream analysis does not.

```yaml
what_changed_2026_08_09:
  before: this catalog specified an analyzer of its own, reporting at warning severity because decision:transport-compatibility-fallback promised a slower path
  now: system:tinybind ships the analysis and a report-only run, and a refusal is a build error, so these entries surface an upstream verdict rather than computing one
  consequence: severity follows the build, and the classifications are upstream's rather than invented here
runner:
  primary: the upstream report-only generation run, which writes nothing and lists every refusal in the package with its chain and remedy
  surfaced_by: api:cli-doctor for a project that declares the second build, reading the report the way it reads any other analysis it did not perform itself
  boundary_kept: requirement:project-diagnostics puts application Go correctness outside doctor; a report doctor relays is not doctor inspecting Go, and the distinction is what keeps the catalog from becoming a linter
  identity: entries stay in the data:diagnostic-check identifier space, so PW0601 is searchable beside PW0412
scope:
  applies_when: data:project-config declares the fasthttp build; a project without it runs none of these
  in: non-test files of application packages
  out: framework packages and generated files, which are the layer that ports
checks:
  refused-transport-occurrence:
    id: PW0601
    trigger: the upstream report refuses a function, for any of its classifications
    severity: error
    reason: the fasthttp build does not compile, so this is what a build failure looks like before the build
    carries: the occurrence position, the chain from the handler that inherited it, and the upstream remedy, none of which this framework rewords
    remedy_this_framework_owns: a refusal naming a pw call is requirement:pw-call-registration, not an application defect, and the message must say so rather than sending a developer to change their handler
  unregistered-framework-call:
    id: PW0602
    trigger: a refusal whose unrecognized callee is a pw function
    severity: error
    audience: this framework rather than the application, so it fails this repository's own verification run and never reaches a user who cannot act on it
    reason: separating it from PW0601 is what keeps a framework omission from being reported as a user's mistake
  mixed-authored-file:
    id: PW0603
    trigger: an authored file holding a transport handler beside a type, const, or var declaration both builds need
    severity: error
    reason: the file granularity of decision:transport-source-transform, which a build tag cannot split
    remedy: move the handler into a file of its own, which is what the scaffolds must already produce
  transport-value-escape:
    id: PW0604
    trigger: a transport value assigned, captured, address-taken, or type-asserted, which the upstream classifier refuses by name
    severity: error
    also_true_without_the_second_build: a pooled request value reused after the handler returns is a defect on its own terms, which is why this one is worth reporting to a project that never declared the second build
    note: this is the only entry here with a reason that survives the fasthttp build being abandoned
rules:
  - a check here relays an upstream verdict and computes none of its own, so the two can never disagree about what a handler does
  - severity is data on the check, per data:diagnostic-check, and every entry here is an error because the build is
  - a project that declares no second build sees none of this, so the option stays free to not take
  - the report is read-only, matching the requirement:project-diagnostics stance, and no entry rewrites application code
deferred:
  reading_the_request_directly: r.URL, r.Header, and r.Form are read through a named selector the upstream rewrite table already covers, so there is nothing here to report; the enumeration that was deferred as noise turned out to be upstream's rewrite table instead
```
