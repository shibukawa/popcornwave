---
id: decision:generated-public-asset-version-control
type: decision
title: Generated Public Asset Version Control
---
The generated subtree of the authored public directory is excluded from version control in an application and committed in a package, on the same reasoning policy:generated-artifacts already applies to generated Go.

```yaml
status: accepted
state: implemented
implementation:
  scaffold: the application .gitignore api:cli-init writes, keyed off the same extractedAssetDir constant the extraction writes to, so the rule cannot drift from the producer
  package_scaffold: unchanged, and the absence is now commented rather than merely true
  guard: >
    one test asserts both directions, because the package scaffold began as a copy
    of the application one and has inherited a rule it should not have had once
    before; a second asserts the ignore line covers the configured Tailwind output,
    since those two constants live in different files and nothing else pairs them
  migrated: auction, helloworld, and live_render, five files untracked and left on disk
  safe_because: all three embed dist/public, so no bare-clone build reads the excluded tree
subject:
  path: public/generated
  writers:
    tailwind: requirement:tailwind-css-integration, whose default output is public/generated/app.css
    extraction: requirement:component-asset-extraction, whose content-hashed styles and scripts land in the same directory
  why_one_directory: >
    the extraction writes where flow:tailwind-css-build already writes, so nothing
    new has to be embedded or served; the placement is deliberate and this decision
    does not move it
  not_subject:
    - the authored files of concept:derived-public-tree, which are input and are committed
    - public-external, the second authored tree, which is not build output at all
    - dist, already excluded entire except the dist/public/.keep sentinel
the_test:
  rule: not "is it generated" but "does something regenerate it from committed sources"
  regenerated: api:cli-generate rebuilds every file under public/generated from the authored CSS entry, the templates, and the scanned Go
  precedent: >
    the same test already decides devbox.d the other way, and the scaffold says so
    in a comment: it is generated output, but nothing regenerates it, so a commit
    is the only thing that can carry it to the next checkout
  consequence: a file that passes the test is excluded; one that fails it is committed whatever produced it
application_and_package:
  application: excluded, per policy:generated-artifacts
  package: committed, per decision:committed-package-artifacts, which already states it for extracted assets
  unchanged: the package inversion and its reasoning; this decision only supplies the application half a rule it never had
why_it_was_missing:
  cause: >
    the exclusion policy is written about generated Go and names one ignore rule,
    **/*_pw_gen.go, which no .css or .js can match; nothing widened it when a
    second artifact class started landing in the tree
  three_way_disagreement:
    component_extraction: calls an extracted file a policy:generated-artifacts artifact, which implies exclusion
    tailwind: says generated CSS is reproducible but is not policy:generated-artifacts generated Go, which implies nothing
    policy: scopes itself to generated Go and rules on the Go ignore line only
  resolution: >
    the policy governs ownership of every generated artifact; the file-name ignore
    rule it carries is one class's instance of a rule this decision states for the
    other class
observed_before_deciding:
  measured_on: examples/auction, 2026-08-12
  committed: 1212 lines
  two_working_trees: 1132 and 1216 lines, from the same commit, neither matching it
  cause: >
    Tailwind emits only the utilities its scan finds, so the output tracks the
    scanned sources; the committed copy still carries .collapse, .absolute, and
    .relative, which occur zero times in the sources it is generated from
  reading: >
    concept:derived-public-tree predicted this as "a sibling written into the
    authored tree is stale forever"; the prediction is now measured rather than
    anticipated
  second_symptom: an extracted asset is content-hashed, so a changed component orphans the previous file rather than replacing it
cost_accepted:
  what: a project using Tailwind needs the pinned CLI to build, since decision:tailwind-host-toolchain never installs one implicitly and rejects a missing one
  why_it_is_free: >
    dist/public is already excluded down to a sentinel and is what the embed
    reads, so a bare clone could not serve an example before this either; the bar
    does not move
  not_free_for: a project that has no Tailwind and only extracted assets, which still needs api:cli-generate before a build; that is already true of *_pw_gen.go
scaffold_obligation:
  where: the .gitignore api:cli-init writes
  line: public/generated/
  why_scaffolded: >
    an author cannot be expected to derive this from two requirements that
    disagree, and a project that starts without the line reproduces the drift
    measured above
migration:
  projects: the examples tracking the files today, which are auction, helloworld, and live_render
  action: add the ignore line and git rm --cached the tracked files
  regenerate_check: api:cli-generate --check still compares a working tree, so excluding the files does not weaken rule:project-integrity-checks
rejected_alternatives:
  commit_and_regenerate_in_ci:
    why_not: it makes every unrelated branch carry a CSS diff, which is the noise decision:committed-package-artifacts accepts only because a package consumer has no other option
  move_the_directory_under_dist:
    why_not: >
      the extraction computes the URL at generation time and the tree is embedded
      and served from the authored side; moving it is a delivery change, not a
      version-control one, and concept:derived-public-tree is where that belongs
  ignore_only_app_css:
    why_not: it fixes the file that happens to hurt today and leaves the content-hashed extracted assets to accumulate as orphans
related:
  - policy:generated-artifacts
  - decision:committed-package-artifacts
  - requirement:tailwind-css-integration
  - requirement:component-asset-extraction
  - concept:derived-public-tree
  - decision:derived-tree-development
  - decision:tailwind-host-toolchain
  - decision:development-public-assets
  - requirement:public-asset-delivery
  - api:cli-init
  - api:cli-generate
  - rule:project-integrity-checks
acceptance:
  - a project api:cli-init scaffolds carries public/generated/ in its .gitignore
  - no example tracks a file under public/generated
  - a component package still commits its extracted assets, unchanged
  - api:cli-generate --check passes on a fresh clone after one generate run
  - public-external and the authored files of public stay committed
```
