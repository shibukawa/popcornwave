---
id: flow:public-asset-build
type: flow
title: Public Asset Build Flow
---
The host build prepares deterministic Zstandard sidecars before Go embeds the complete public tree.

```yaml
trigger:
  - api:cli-build
steps:
  - walk project-root public without following symbolic links
  - reject irregular files and exclude generated .zstd sidecars from source discovery
  - select policy:public-asset-precompression eligible source files
  - write each compressed representation atomically as {source}.zstd
  - remove stale generated .zstd sidecars whose source is absent or ineligible
  - verify the api:cli-init scaffolded project-root public.go exists
  - compile only after every sidecar is current
inputs:
  empty_tree_sentinel: public/.keep is embedded but never served
  ignored_as_sources:
    - public/**/*.zstd
    - public/.keep
public_go:
  - is scaffolded once by api:cli-init and never regenerated
  - uses //go:embed all:public with embed.FS
  - exposes PublicFS() fs.FS rooted at public
  - is imported by cmd/myapp/main.go and passed through api:application-lifecycle
reproducibility:
  encoder: requirement:contrib-zstd host implementation
  settings: fixed level, concurrency, window, and frame-checksum policy
failure:
  - preserve the previous complete sidecar
  - prevent build from embedding mixed or stale representations
exclusion: api:cli-dev follows decision:development-public-assets
```
