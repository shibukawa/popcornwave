---
id: decision:derived-tree-development
type: decision
title: Derived Tree in Development
---
api:cli-dev runs the same transforms as a production build and serves concept:derived-public-tree from disk, because a development loop that skipped them would serve URLs the templates no longer name.

```yaml
status: proposed
supersedes_on_acceptance: decision:development-public-assets, on its source directory, its forced local read, and its no-conversion rule
why_the_old_rule_breaks:
  reference_moved: a template names app.ts and the page names app.js, so serving the authored tree returns 404 for every rewritten reference
  rewrite_is_compiled: the rewrite lands in generated Go, so a development-only skip makes generated output differ by environment and policy:generated-artifacts --check compares one profile
  conclusion: the switch cannot be per environment; only the settings behind it can be
what_keeps_it_affordable:
  names_ignore_settings: an output name derived from its source is identical under any quality profile, so a cheap development encode produces the same generated Go as a production one
  conversion_cache: an outcome, including a decision to decline, is reused across runs, so a warm loop converts nothing
  incremental: a build tool that rebuilds only what changed carries the javascript case, per decision:asset-transform-toolchain
  first_run_cost: a cold cache converts serially, which is the one visible regression against today's loop
loop_changes:
  watch: the authored public tree and every asset source enter the watch set, where decision:developer-loop-watch-scope excludes public today
  rebuild_scope: an asset change rebuilds the tree and the manifest and needs no Go rebuild, unless a rewritten reference changed, which regenerates
  serving: read dist/public from the working directory, so an edit is visible without an application rebuild, which is what the old rule bought and this keeps
  precompression: no .zstd sidecar is produced or served in development, unchanged
  negotiation: the default representation only; policy:public-asset-media-negotiation does not run
  manifest: read from disk rather than embedded, so a rebuilt tree needs no restart
rules:
  - path and file security remain policy:public-asset-resolution
  - a missing entry is 404 even when an older embedded file exists, unchanged
  - the authored tree is never served, so a file that failed to convert is absent rather than silently original
open_questions:
  - whether a first run should convert eagerly or lazily on first request, which trades startup for a stall inside the loop
  - whether a shared conversion cache lives in the project, the module cache, or a configured path
```
