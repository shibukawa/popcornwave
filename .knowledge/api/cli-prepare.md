---
id: api:cli-prepare
type: api
title: pw prepare
---
pw prepare runs every rule:container-build-inputs host step and stops before the compiler, so a build the framework does not drive has one command to call instead of a sequence to reproduce.

```yaml
usage: pw prepare
definition: api:cli-build without its final compiler invocation, which is the whole contract
steps:
  - api:cli-generate
  - flow:tailwind-css-build in production mode, only when Tailwind is enabled
  - flow:public-asset-build
  - the development-only import rejection over data:project-config project.main, which refuses requirement:contrib-devidp and the other packages api:cli-build already refuses
relationship:
  pw_build: api:cli-prepare followed by the compiler, so the two commands cannot drift in content or in order
  pw_generate: the first step alone, kept for the editor and for the --check gate CI runs; a caller that wants a compilable tree wants this command instead
callers:
  - Dockerfile.tinygo, per decision:explicit-tinygo-compile-step
  - a cross-compiled or otherwise custom go build the operator drives with their own flags
  - requirement:dockerless-image-builders, whose builders own the compile step and would otherwise each need the sequence written out
scope:
  compiles_nothing: the command produces no binary and reports no compiler diagnostic
  toolchain_agnostic: every step is host Go per decision:host-tools-target-runtime, so the output serves either compiler and the command takes no toolchain argument
  leaves_dist: flow:public-asset-build creates dist/public even for a project with no public asset, because public.go names it in a go:embed directive
naming:
  chosen: prepare, a verb for everything that happens before the compiler
  assets_rejected: it would name two of the four steps and hide the import rejection, which is a security gate rather than an asset step
  flag_rejected: pw build --no-compile reads as a build that was asked not to build, and the caller here is not building at all
reporting:
  policy: policy:cli-progress-reporting
  phases: the api:cli-build phases up to the compiler, so a reader who has seen one recognizes the other
documentation:
  page: its own pw command page, since every other command has one and the callers above are three different situations rather than a footnote to api:cli-build
  carries: the pw generate trap, which is the mistake this command exists to prevent — that command writes the generated Go but not dist/public, so its output does not compile
  cited_by: requirement:container-deployment-docs for Dockerfile.tinygo, and the api:cli-build page for the TinyGo and cross-compiling cases it no longer explains itself
exit:
  success: 0
  any_step_failed: nonzero, with that step's diagnostic unchanged
```
