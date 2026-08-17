---
id: rule:project-integrity-checks
type: rule
title: Project Integrity Checks
---
The PW01xx data:diagnostic-check entries: whether the project's declared shape, its toolchain, and its generated artifacts still agree with each other.

```yaml
project_shape:
  main-package-missing:
    trigger: data:project-config project.main names a path that does not exist or is not a main package
    severity: error
    reason: every other host command builds it, so this failure is reported once here rather than as five confusing ones
  migration-dir-missing:
    trigger: data:project-config migration.dir names a directory that does not exist while the database capability is present
    severity: error
    remedy: api:cli-add database, which writes the directory and its starter schema
  generate-purpose-empty-with-sources:
    trigger: a generate purpose lists no directory while sources of its kind exist in the project
    severity: warning
    reason: decision:explicit-generation-sources means an unlisted directory is silently not generated from
generated_artifacts:
  orphan-generated-file:
    trigger: a {source-base}_pw_gen.go whose .pw.html or .pw.sql source no longer exists
    severity: error
    reason: the orphan still compiles and its registrations still run, so a deleted page keeps serving and a deleted query keeps building; nothing else in the toolchain reports this
    remedy: delete the generated file, which api:cli-generate does for a source inside its purpose
    note: this is the failure mode most specific to generating beside the source, which is why it is an error rather than a warning
  generated-older-than-source:
    trigger: a generated file older than the source it was generated from
    severity: warning
    remedy: pw generate
    relation: api:cli-check is the authority on content drift; this check is the cheap timestamp form that also fires when the check cannot run
  generated-outside-purpose:
    trigger: a generated file, .pw.html, or .pw.sql outside every generate purpose
    severity: warning
    reference: the same condition api:cli-generate warns about, reported here so one command sees it
  generated-files-not-ignored:
    trigger: the project is a git work tree and *_pw_gen.go is neither ignored nor tracked consistently
    severity: note
    reason: policy:generated-artifacts makes them reproducible output, and a project should say once whether it commits them
public_assets:
  asset-content-type-mismatch:
    trigger: an authored public file whose bytes contradict its extension, per policy:asset-content-signature
    severity: error
    remedy: rename the file to the type it actually is, or exempt the path in assets.verify.allow
    reference: requirement:asset-content-verification
    reason: api:cli-build fails on the same condition, so this is the form that reports without a build and the only one that sees a read_local tree no build validated
  svg-active-content:
    trigger: an authored .svg whose bytes carry a literal policy:svg-active-content scans for
    severity: error
    remedy: remove the script, or exempt the path in assets.verify.allow when the svg is interactive on purpose
    reference: requirement:asset-content-verification
    reason: the sandbox header already neutralises the file at the browser, so this check exists to tell an author what they committed rather than to hold the boundary
  embedded-large-media:
    trigger: a file of an already-compact kind above 4 MiB in the embedded public tree
    severity: warning
    remedy: move it to the external tree of requirement:external-public-assets, which ships beside the binary
    reason: public is compiled into the executable, and a threshold that only decides whether to speak is safe where one deciding the location would make the same asset embedded in one build and external in the next
toolchain:
  go-version-mismatch:
    trigger: the Go version in devbox.json disagrees with the go directive in go.mod
    severity: warning
    reason: the build that succeeds in the shell and the build that succeeds in CI are then different builds
  tinygo-baseline-unmet:
    trigger: data:project-config project.toolchain is tinygo and the pinned TinyGo version is below decision:tinygo-042-baseline, or its supported host Go range excludes the pinned Go
    severity: error
    reference: rule:tinygo-runtime-compatibility
  outside-devbox-shell:
    trigger: devbox.json exists and the current process environment is not the devbox shell
    severity: note
    reason: api:cli-dev expects the tools that shell provides, so a missing tool later is easier to read as this
  declared-service-missing:
    trigger: configuration selects a service, such as a Valkey endpoint or a session backend needing one, while devbox.json declares no such service
    severity: warning
    remedy: api:cli-add redis-valkey
    reason: api:cli-add and doctor are a pair; a capability added by hand is exactly where configuration and dependency separate
  tailwind-toolchain-missing:
    trigger: data:project-config assets.tailwind.enabled is true while devbox.json pins no decision:tailwind-host-toolchain package, or the configured input file does not exist
    severity: error
    reference: requirement:tailwind-css-integration
  port-unavailable:
    trigger: the configured server port is already bound on loopback
    scope: the dev token only, because another host's port says nothing about this one
    severity: warning
    bound: one non-blocking local check; no remote address is contacted
rules:
  - every check here reads files and the process environment only, per decision:host-side-diagnostic-analysis
  - a check about a capability is skipped when requirement:incremental-project-capabilities reports the capability absent, so a project without a database is not told about migrations
  - no check inspects application Go code for style or correctness, per the requirement:project-diagnostics scope boundary
```
