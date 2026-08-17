---
id: requirement:deployed-debug-information
type: requirement
title: Debug Information In A Deployed Artifact
---
Whether a deployed artifact carries debug information must be a property of the build invocation rather than a project setting, because a deployment that rehearses production and a deployment that is being debugged want opposite answers from one repository at one commit.

```yaml
status: requirements recorded 2026-08-10, unimplemented
priority: should
source: user request and design discussion 2026-08-10
driving_case:
  what: the linked source map the script build of requirement:derived-asset-pipeline
    emits reaches every deployed artifact today, and nothing decides otherwise
  why_it_matters: >
    the map carries sourcesContent, verified 2026-08-10 by building an entry that
    imports a module and finding the authored body of both files inside the map.
    A production deployment serves the authored TypeScript, with its comments and
    its names, to anyone who asks
  reach: the map is a produced file, so it lands in concept:derived-public-tree,
    takes an entry in data:public-asset-manifest, and is served like any other
    declared URL
environment_classes:
  local: api:cli-dev. Debug information on, always
  shared_test_or_cd: a deployed artifact being debugged by more than one person.
    Debug information on, deliberately
  staging_and_prod: >
    debug information off, and the two are identical. A staging artifact that
    differs from the prod artifact is not a rehearsal of prod, which is the
    user's stated reason for the requirement
  consequence: >
    three classes, two artifact shapes. Staging and prod share one because they
    must; the shared test environment is deliberately its own and nobody claims
    it rehearses anything
build_modes_today:
  verified: read against the tree on 2026-08-10
  count: two, and neither is a setting
  pwdev: a Go build tag. api:cli-dev runs go run -tags=pwdev, as the storybook
    runner and the migrate DSN print do
  release: api:cli-build runs a plain go build with -trimpath and -ldflags=-s -w
    and passes no tag
  no_debug_build: api:cli-build accepts no arguments at all and refuses any, so
    the shared-test artifact cannot be asked for today
  who_produces_the_tree: api:cli-dev, api:cli-build, and api:cli-generate, all
    through one function reading assets of data:project-config. The asset build
    takes no environment and no mode input of any kind
shapes: concept:build-artifact-shapes holds what each artifact carries and which
  gate decides it; this requirement decides only the debug-information gate
where_the_switch_cannot_live:
  runtime_env_config: >
    config.<env>.toml is read by the running application, and pw build reads no
    APP_ENV. A runtime token cannot decide what a binary contains. Gating only
    the serving leaves the artifact carrying the sources, so the cost is still
    paid and anyone holding the binary holds the map
  build_tag_alone: >
    pwdev is the development runtime, not a debug-information level. It carries
    the error overlay, the launcher, the dev-only modules, and the DSN print,
    and policy:devidp-safety locks other things to it. A shared test deployment
    wants symbols, not the development identity provider
decided_shape:
  form: an explicit debug flag on the build invocation, off by default
  on: api:cli-build and api:cli-generate both, not just the first
  why_the_generate_command_too: >
    api:cli-generate is api:cli-build without the compile, and it exists for
    builds this project does not drive: the TinyGo Dockerfile, a cross-compiled
    go build with the operator's own flags, an image builder owning the compile
    step. Those are the container deployment paths, so a flag only api:cli-build
    understands misses the case most likely to be a real deployment
  why_default_off: >
    the failure being fixed is silence. A default that can be flipped once and
    forgotten reproduces it, and a CI pipeline that omits a flag should get the
    safe artifact rather than the debuggable one
  why_not_three_values: >
    one flag with a default expresses the three environment classes already:
    api:cli-dev is the local class, the flag is the shared-test class, and its
    absence is staging and prod
per_shape_defaults_in_project_config:
  proposal: assets keys in data:project-config stating what each deployed shape
    carries, one for the release shape and one for the debug shape
  why_this_works_where_one_key_did_not: >
    a single boolean could not express the axis, because it would have had to
    mean one thing for api:cli-dev and another for api:cli-build. Keyed per
    shape, the invocation selects which key applies and the file states both
    without contradiction
  what_it_buys:
    reviewable: a deployment carrying its sources becomes a line in a file rather
      than a property of whoever ran the pipeline
    real_overrides: >
      a project shipping symbols to a crash reporter wants the release shape to
      keep Go debug information, and a project treating its shared test
      environment as production-shaped wants the debug shape to keep none. Both
      are stated rather than argued about
  keeps_the_default: the flag's default stands when neither key is present, so a
    project that says nothing gets the safe artifact
  residual_risk: >
    a project can still set the release shape to keep source maps, which is the
    thing this requirement removes. That is accepted: an explicit line someone
    reviewed is a different failure from one nobody could see
what_the_flag_governs:
  source_maps: >
    the driving case. Emitted and shipped under the flag, absent without it
  go_debug_information: >
    api:cli-build strips DWARF and the host symbol table unconditionally today
    while retaining pclntab. That is the same axis, and a shared test artifact
    wanting a source map wants a stack trace with file and line too
  open_whether_to_move_ldflags: >
    changing when -ldflags=-s -w is passed is beyond the source-map request that
    started this, and it changes the artifact of every existing project. Recorded
    as the sibling member rather than decided
  not_governed:
    minification: >
      a bundle stays minified either way. The map is what makes a minified bundle
      debuggable, so unminifying is a second decision with no case behind it
    development_runtime: >
      nothing here turns on pwdev. The error overlay, the launcher, and the
      development identity provider stay locked to api:cli-dev
must:
  - drop the trailing sourceMappingURL comment with the map. A bundle naming a
    map the tree does not hold makes every devtools open a 404, which is worse
    than the map it replaced
  - carry the flag in the conversion CacheKey params. The script hook keys on the
    esbuild version alone today, so flipping it without that replays the bundle
    built under the other setting
  - drop the map's data:public-asset-manifest entry and its
    policy:public-asset-precompression sidecar with the map, because the manifest
    is the only thing that makes a URL exist and one naming an absent file is a
    declared 404
  - keep api:cli-dev opening devtools onto the authored TypeScript, which is the
    whole reason the map is emitted at all
  - make the artifact say which shape it is, so a deployed binary can be asked
    rather than inferred from whoever ran the pipeline
hashed_name_decision:
  today: >
    the bundle digest is taken over the body without its sourceMappingURL
    comment, and the comment is written back afterwards, so the hashed js name is
    identical whether or not a map was emitted
  therefore: >
    a debug artifact and a stripped artifact name one URL for two byte sequences,
    differing only by a trailing comment that changes no behavior
  option_keep: >
    the rewritten reference compiled into generated Go is identical across
    shapes, so policy:generated-artifacts --check stays unforked. Residual risk
    is one immutable URL holding two byte sequences, which only bites where a
    cache spans a debug and a stripped deployment
  option_include_comment_in_digest: >
    honest URLs, at the cost of generated output that differs per shape, which
    that policy compares against one profile only
  recommendation: keep today's behavior and record the residual risk, because the
    generated-artifact fork is the more expensive of the two
honesty:
  what_this_buys: the authored TypeScript with its comments and names stops being
    served by staging and prod, and those artifacts lose the bytes
  what_it_does_not_buy: >
    secrecy. A minified bundle is still readable and still describes what it
    does, so nothing here protects client code from being read
acceptance:
  - a plain pw build artifact holds no map under the derived tree, and no bundle
    in it carries a sourceMappingURL comment
  - a staging artifact and a prod artifact built from one commit are
    byte-identical, which is the user's stated reason for the requirement
  - the same commit built with the flag serves a map and resolves a browser stack
    trace to the authored TypeScript
  - api:cli-generate honors the flag, so the container path can produce either
    shape
  - pw dev still resolves a stack trace to the authored TypeScript line
  - flipping the flag rebuilds the bundle rather than replaying the cached one
  - the manifest of a stripped artifact declares no URL its tree does not hold
open:
  flag_name: >
    --debug reads as the Go debug information it does not govern yet. Whatever it
    is called has to still be right if the ldflags sibling moves under it later
  ldflags_sibling: whether the same flag stops stripping DWARF and the symbol
    table, which changes the artifact of every existing project
  doctor_check: >
    whether rule:production-readiness-checks should report a map found in a tree
    built without the flag. It is the check that would have caught this before a
    user did, and it costs one extension walk
out_of_scope:
  - the css minify, which emits no map
  - any form of obfuscation or client-code protection
  - a per-environment build config; the environment token stays runtime-only
  - authored positions for .pw.html and .pw.sql errors, declined 2026-08-10. Go
    records those through //line directives, which land in pclntab and are
    already in every artifact, so they would never have been on this axis

