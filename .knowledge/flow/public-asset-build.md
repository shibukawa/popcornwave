---
id: flow:public-asset-build
type: flow
title: Public Asset Build Flow
---
The host build prepares deterministic brotli, Zstandard, and gzip sidecars before Go embeds the complete public tree.

```yaml
trigger:
  - api:cli-build
steps:
  - walk project-root public without following symbolic links
  - reject irregular files and exclude generated sidecars of every coding from source discovery
  - select policy:public-asset-precompression eligible source files
  - write each coding's representation atomically as {source}.br, {source}.zstd, and {source}.gz
  - skip a coding whose result is not smaller than the source, and report the skip
  - remove stale generated sidecars whose source is absent, ineligible, or newly skipped
  - verify the api:cli-init scaffolded project-root public.go exists
  - compile only after every sidecar is current
inputs:
  empty_tree_sentinel: public/.keep is embedded but never served
  ignored_as_sources:
    - public/**/*.br
    - public/**/*.zstd
    - public/**/*.gz
    - public/.keep
public_go:
  - is scaffolded once by api:cli-init and never regenerated
  - uses //go:embed all:public with embed.FS
  - exposes PublicFS() fs.FS rooted at public
  - init registers PublicFS through api:application-lifecycle
  - generated main bootstrap blank-imports the public package
reproducibility:
  encoders: the three of policy:public-asset-precompression, each pinned to fixed settings
  settings: fixed level, concurrency, window, and frame-checksum policy per coding
  determinism: identical source bytes produce identical sidecar bytes for every coding, so policy:generated-artifacts --check compares them
cost:
  where_it_lands: build time only, which is what makes maximum levels affordable and brotli usable at all
  brotli_note: the maximum brotli level is roughly two orders of magnitude slower than the others per byte, so it dominates the sidecar step on a large tree
failure:
  - preserve the previous complete sidecar
  - prevent build from embedding mixed or stale representations
  - a single coding failing does not fail the build; the file keeps the codings that succeeded, per policy:public-asset-negotiation fall-through
exclusion: api:cli-dev follows decision:development-public-assets
```
