---
id: requirement:project-diagnostics
type: requirement
title: Project Diagnostics
---
One command must report what the project will actually run in a named data:runtime-environment: which features are on, what implements them, what wiring or artifact is missing or stale, and which resolved value is inadvisable there.

```yaml
motivation:
  - feature state is spread over decision:independent-runtime-config-bindings targets, blank imports per decision:import-registered-session-plugins, and four precedence layers, so no single file answers "what is on"
  - a missing plugin or database/sql driver import surfaces only as a startup error whose remedy is an import line the message does not carry
  - policy:startup-summary reports resolved values but only for a process that already started, and only for the environment it started in
  - policy:query-log-safety, policy:devidp-safety, and decision:development-public-assets each bound a development-only setting, and an operator has no way to check them all before deploying
  - a setting that is merely inadvisable outside dev is not a startup error, so nothing reports it today
  - a secret that is valid but fixed in a committed file starts cleanly, and a running process cannot tell that its value came from a file every developer can read
  - dev may authenticate against requirement:contrib-devidp, so the same auth section is correct in one environment and an outage or a hole in another
  - generating beside the source means a deleted page can leave a compiling artifact that still registers its route, and no other tool in the toolchain reports it
  - api:cli-add installs a capability's configuration and its dependency together, so the projects that drift are the ones edited by hand; add and doctor are a pair
  - a pre-launch checklist held in documentation goes stale silently, while the same list expressed as checks fails when it is wrong
scope:
  command: api:cli-doctor
  analysis: decision:host-side-diagnostic-analysis
  report: data:diagnostic-report
  check: data:diagnostic-check
  ownership: decision:shared-check-catalog
  catalogs:
    - rule:project-integrity-checks
    - rule:route-and-template-checks
    - rule:storage-checks
    - rule:configuration-advisories
    - rule:production-readiness-checks
  flow: flow:project-diagnosis
boundary:
  in: framework conventions, configuration, wiring, generated artifacts, routes, and migration sources
  out: the correctness or style of application Go code, which is what go vet and a linter already own
  reason: naming the boundary is what keeps the catalog from growing into a second linter
behavior:
  - take the environment to diagnose as an option, so any environment can be examined from any host
  - merge typed defaults with the TOML file that environment selects, the way api:runtime-configuration would for its file layer
  - report every discovered binding, its owner, and the place of every field
  - report each framework feature as on or off with the key that decided it and the implementation behind it
  - name the concrete remedy for every gap, as an import path, a configuration key, or an api:cli-add capability
  - evaluate every check against the diagnosed token rather than against a fixed opinion of production
  - carry a stable identifier and a documentation link on every finding, so it can be searched and cited
  - build nothing, start nothing, and write nothing, and connect to nothing unless asked
  - report what the analysis cannot determine, and which checks it therefore did not run, instead of assuming a default
acceptance:
  - a project whose selected session backend plugin is not imported reports one error naming the import line
  - a DSN whose database/sql driver is not linked reports the driver package for that scheme
  - a TOML section owned by an unimported plugin is reported as a wiring gap, not as an unknown key
  - pw doctor --env prod on a development host reports the advisories that would apply in production
  - query diagnostics left on outside dev is reported as a warning naming the environment, bind values, and threshold
  - a secret-classified field carrying a fixed literal in a non-dev file is an error naming the key and the environment variable that should carry it, and the report shows no value
  - the scaffolded placeholder secret left in place is an error outside dev, and one literal secret shared by two environment files is an error when either is not dev
  - auth enabled for stg or prod with a loopback or http issuer is an error, while the same issuer in dev is reported as the requirement:contrib-devidp arrangement it is
  - auth enabled for a non-dev token with no determinable provider values is a note naming the environment variables the deployment must set, not a false error
  - a generated file whose .pw.html or .pw.sql source was deleted is an error, since it still compiles and still registers
  - two registrations of one pattern are reported as an error before the api:serve-mux panic that would otherwise find them at startup
  - an application pattern colliding with an enabled framework mount names the configuration key that enabled the mount
  - a project whose devbox pin and go.mod directive disagree is a warning, and a Tailwind-enabled project with no pinned toolchain is an error
  - without the online option the migration section states the pending count as unanswered rather than showing a clean database
  - with the online option, a live schema that no longer matches what the sources produce is reported, including the case of a migration edited after it was applied
  - a clean project exits zero and a project with an error finding exits nonzero
  - a project that does not compile still yields every section the parsable packages support
  - a key whose deployed value arrives from an environment variable is listed as undeterminable, and its checks are reported as suppressed
  - the same run in CI, where those variables exist, turns those suppressions into real verdicts and reports the host mode it had
  - the report contains no DSN credential, no token, no provider secret, and no expanded ${NAME} value
non_goals:
  - a --fix option; the diagnosis stays read-only and remedies stay with api:cli-add, api:cli-generate, and api:cli-migrate
  - linting application Go code, per the scope boundary; rule:transport-handle-checks is the one framework rule about handler source, and it ships as a go vet analyzer for exactly this reason
  - observing a live process; the diagnosis reads the project, and in-process collection is deferred with decision:host-side-diagnostic-analysis
  - replacing startup validation; api:application-lifecycle keeps failing before request acceptance
  - a runtime health surface, which stays with policy:operational-endpoints
  - continuous monitoring, drift alerting, or history between runs
  - auditing application code, dependencies, or infrastructure
  - checking generated-source freshness itself, which api:cli-check already owns and doctor delegates to
```
