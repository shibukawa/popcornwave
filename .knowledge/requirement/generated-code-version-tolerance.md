---
id: requirement:generated-code-version-tolerance
type: requirement
title: Generated Code Must State What Runtime It Links Against
---
Generated code emitted by one generator version must have a stated support window against the runtime versions it may be linked with, because a published package freezes the artifact and the consumer's module graph picks the runtime.

```yaml
owner: system:tinybind
status: not raised upstream as of 2026-08-02
priority: should
why_it_only_appears_now:
  today: an application regenerates before every build, so the generator and the runtime are one dependency at one version and skew is impossible
  with_packages: decision:committed-package-artifacts freezes the artifact at publish time while go.mod resolution picks the runtime at build time, and Go selects the higher of the two
  consequence: the pair that runs was never the pair that was tested, for every consumer whose application resolved a newer version than the package's author did
what_is_missing:
  no_statement: the upstream catalog records per-release compatibility notes for its own features and states nothing about generated output against a different runtime
  observed_variance: generated call shapes have changed inside minor versions; the runtime entry lost a client parameter between two v0.2 releases, which is a breaking change to generated call sites rather than to a public API a human wrote
  unverifiable_here: this framework can pin its own dependency and cannot pin a third-party package's, so policy:package-compatibility states a window over a guarantee nobody made
proposed_shape:
  statement: each release names the generator versions whose output it accepts, as an ordinary compatibility note beside the ones already published
  scope: the runtime entries generated code calls, which is a much narrower surface than the public API and is the only one that has to hold
  breaking_signal: a release that breaks generated output says so, so a downstream can refuse the pair before the compile error
  optional_stronger_form: a constant in generated output naming the generator version, so a runtime can refuse a pair it does not accept with a named error rather than a link failure; a compile error is acceptable if the note exists, and this is only worth it if the note cannot be relied on
what_this_framework_does_meanwhile:
  - records the generating versions in data:component-package-manifest
  - states its own window in policy:package-compatibility, derived from testing rather than from a guarantee
  - reports a package outside the window through api:cli-doctor
  - accepts that a minor upstream release can invalidate the window without warning, which is the residual this requirement exists to remove
acceptance:
  - a release states which generator versions' output it accepts
  - a release that breaks generated output is identifiable before a consumer builds
  - a package generated inside the stated window links and runs against every accepted runtime
related:
  - policy:package-compatibility
  - decision:committed-package-artifacts
```
