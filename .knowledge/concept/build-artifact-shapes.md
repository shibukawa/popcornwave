---
id: concept:build-artifact-shapes
type: concept
title: Build Artifact Shapes
---
A project at one commit produces artifacts of three shapes, and what separates them is four independent gates rather than one level, so a debug artifact is the release artifact plus debug information and never the development artifact with things taken away.

```yaml
verified: read against the tree on 2026-08-10
shapes:
  development:
    produced_by: api:cli-dev
    exists: yes
    carries: >
      the pwdev browser runtime, the disk-served asset tree, the dev data and
      test seams, the DSN print, source maps, and full Go debug information
    alongside_it: the console, the telemetry viewer, the storybook, and the
      development identity provider, none of which are in the binary
  debug:
    produced_by: api:cli-build with an explicit debug flag, per
      requirement:deployed-debug-information
    exists: no, proposed
    carries: source maps and Go debug information, and nothing developmental
    for: a shared test or CD deployment being debugged by more than one person
  release:
    produced_by: api:cli-build or api:cli-generate with no flag
    exists: yes
    carries: neither, and this is the shape staging and prod share
gates:
  pwdev_build_tag:
    decides: what is compiled into the binary
    passed_by: api:cli-dev through go run -tags=pwdev, and the storybook and
      migrate runners
    gates: the browser error overlay and launcher, the dev announce, the dev data
      and dev test seams, the JWT development path, --pw-print-dsn, and
      decision:development-public-assets serving the tree from disk rather than
      from the embed
    invariant: policy:dev-console-boundary admits no development behavior into an
      application except through this tag
  development_import_refusal:
    decides: what may be in a built application's dependency graph at all
    enforced_by: api:cli-build and api:cli-generate, before the compiler runs
    refuses: the development identity provider, the passkey test authenticator,
      and the authentication test seam, each of which authenticates nobody or
      mints credentials a relying party accepts
    independent_of_the_tag: it is a build refusal rather than a compile-time
      exclusion, so it holds whatever tags are passed
  debug_information:
    decides: whether the artifact can be read back to its sources
    today: api:cli-build strips DWARF and the host symbol table unconditionally
      while retaining pclntab, and the script build emits a source map always
    proposed: requirement:deployed-debug-information makes both follow one flag
  runtime_environment:
    decides: nothing about the artifact
    is: the APP_ENV token, which selects config.<env>.toml and gates startup
      relaxations such as policy:devidp-safety refusing to start under prod
    why_it_is_listed: it is the gate most easily mistaken for the others, and it
      cannot decide what a binary contains
why_debug_is_additive:
  claim: a debug artifact is the release artifact with debug information added,
    not the development artifact with development behavior removed
  because: >
    the development console, the telemetry viewer, the storybook, and the
    identity provider are processes api:cli-dev runs beside the application, and
    everything developmental inside the binary is behind the pwdev tag that
    api:cli-build does not pass. A debug build therefore removes nothing, because
    there is nothing in it to remove
  consequence: >
    the flag needs no subtraction logic and no list of things to strip, which is
    the difference between a safe feature and one where a forgotten entry ships
    the development identity provider to a shared environment
```
