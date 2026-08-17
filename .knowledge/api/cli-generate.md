---
id: api:cli-generate
type: api
title: pw generate
---
pw generate runs every rule:container-build-inputs host step and stops before the compiler, so a build the framework does not drive has one command to call instead of a sequence to reproduce.

```yaml
usage: pw generate [--code-only] [--debug] [--backend nethttp|fasthttp]
definition: api:cli-build without its final compiler invocation, which is the whole contract
steps:
  - concept:code-generation
  - flow:tailwind-css-build in production mode, only when Tailwind is enabled
  - flow:public-asset-build
  - the development-only import rejection over data:project-config project.main, which refuses requirement:contrib-devidp and the other packages api:cli-build already refuses
relationship:
  pw_build: api:cli-generate followed by the compiler, so the two commands cannot drift in content or in order
  pw_check: the first step planned and not written, per api:cli-check; it verifies less than this command writes
code_only:
  runs: concept:code-generation alone
  writes_no: asset tree, stylesheet
  keeps: the development-only import rejection, because the steps this flag skips are the ones that write files and a flag must not also be the way past a security gate; its cost is a dependency-graph listing rather than a compile
  for: the requirement:editor-tasks generate command, and a developer who wants generated Go without waiting for a minified stylesheet
  not_for: a tree handed to a compiler, which is the unflagged command
  refuses_debug: --debug survives only as source maps in an asset tree this flag does not build, so the combination is rejected rather than half-honoured
project_kind:
  application: every step
  package: concept:code-generation and nothing after it, because a concept:component-package has no entry point whose imports could be rejected, no public.go to embed a tree for, and no document shell to style; the same result --code-only gives, selected by data:project-config project.kind rather than by a flag the author has to know to pass
  contrast: api:cli-build and api:cli-dev are refused in a package project, and this command is not — it is how a package rebuilds the artifacts decision:committed-package-artifacts commits
flags:
  debug: keeps the source maps in the built tree, exactly as api:cli-build --debug does; the linker half belongs to the compiler line the caller writes, per requirement:deployed-debug-information
  backend: selects the build tags the dependency safety check lists the graph under
  no_target: provider packaging belongs to api:cli-build, so --target is refused here
callers:
  - Dockerfile.tinygo, per decision:explicit-tinygo-compile-step
  - a cross-compiled or otherwise custom go build the operator drives with their own flags
  - requirement:dockerless-image-builders, whose builders own the compile step and would otherwise each need the sequence written out
  - api:cli-build, which is this command plus the compiler
scope:
  compiles_nothing: the command produces no binary and reports no compiler diagnostic
  toolchain_agnostic: every step is host Go per decision:host-tools-target-runtime, so the output serves either compiler and the command takes no toolchain argument
  leaves_dist: flow:public-asset-build creates dist/public even for a project with no public asset, because public.go names it in a go:embed directive
naming:
  chosen: generate, per requirement:cli-generate-check-rename — the name every caller guesses for "make a tree the compiler can read", given to the command that actually leaves one
  was: prepare, while the narrower generation held the name generate; a tree prepared with that one failed to compile on a go:embed over a directory nothing built, and the documentation carried a section per page warning about it
  prepare_rejected: nothing in the word names any of the four steps, so a caller who had not read the page could not tell it from generate
  flag_rejected: pw build --no-compile reads as a build that was asked not to build, and the caller here is not building at all
reporting:
  policy: policy:cli-progress-reporting
  phases: the api:cli-build phases up to the compiler, so a reader who has seen one recognizes the other
documentation:
  page: its own pw command page, since every other command has one and the callers above are three different situations rather than a footnote to api:cli-build
  carries: what --code-only leaves out, which is the one way left to get a tree that does not compile
  cited_by: requirement:container-deployment-docs for Dockerfile.tinygo, and the api:cli-build page for the TinyGo and cross-compiling cases it no longer explains itself
exit:
  success: 0
  any_step_failed: nonzero, with that step's diagnostic unchanged
```
