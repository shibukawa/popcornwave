---
id: decision:package-contract-split
type: decision
title: Package Contract Split
---
A concept:component-package declares itself twice, in Go for the runtime and in a manifest for the tooling, because the two consumers read at different times and neither can read the other's form.

```yaml
status: accepted
question: whether a package is described by an entry point function, by a generator manifest, or by both
audiences:
  runtime:
    reader: the linked binary
    needs: assets, extension registration, and the identity a served URL is derived from
    timing: package initialization, inside a process that has already compiled everything
    form_it_can_read: Go values, because rule:tinygo-runtime-compatibility forbids reflection and a TinyGo target may have no filesystem
  tooling:
    reader: api:cli-generate, api:cli-migrate, and api:cli-doctor in the consumer project
    needs: the import path to link, the migration stream to locate, generator versions to compare, and the required capabilities to check
    timing: before the consumer compiles, and specifically while generating the file that performs the import
    form_it_can_read: a file in the module cache, because compiling a dependency to ask it a question is a build to answer a prompt
    circularity: the strongest case; api:cli-generate must know what to import in order to emit the import, so the answer cannot come from a linked binary at any price
why_not_one:
  manifest_only:
    fails: the runtime needs an fs.FS of embedded assets, and a manifest cannot carry bytes into a TinyGo binary
    fails_also: a manifest-driven runtime is a registry keyed by strings, which is the reflection-free rule this framework keeps
  entry_point_only:
    fails: decision:declared-package-installation makes the generated blank import the install, and a generator cannot read a registration from a package it has not yet caused to be linked
    fails_also: a Go source scan of a dependency is the whole-project discovery decision:explicit-generation-sources already replaced, and it would cross a module boundary to do it
    fails_also_2: api:cli-migrate applies a package's stream without an application binary, per decision:goose-migration-engine, so it cannot ask a process that does not exist
    partially_true: a linked application could expose its own registrations through a generated subcommand, but that answers only after the import, which is one step too late
division:
  api:package-registration: what the process needs and what only bytes can carry
  data:component-package-manifest: what a command needs before there is a process, on both sides; the package publishes what it offers and the project declares what it uses
  overlap: module path and version appear in both, and a mismatch is an api:cli-doctor finding rather than a runtime error
  rule: no fact is authored twice; the manifest states the module path once and the Go value repeats it as the identity it registers under
format:
  chosen: TOML, in the package's own data:project-config file under a package section
  rejected_json: the project already reads TOML for both tooling and runtime configuration, and a second syntax buys nothing
  rejected_separate_file: a second file drifts from the first, and both would be read by the same command in the same directory
  reachability: go.mod ships every file in the module directory, so the consumer's CLI reads the manifest from the module cache with no network access
precedent:
  api:framework-extension: an imported package contributes middleware from its init without pw importing it, which is this split's runtime half already in service
  bootstrap_linker: concept:project-layout already generates a blank-import file into the main package to link registration packages, which is the mechanism decision:declared-package-installation extends rather than a new one
```
