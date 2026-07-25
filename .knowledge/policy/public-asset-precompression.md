---
id: policy:public-asset-precompression
type: policy
title: Public Asset Precompression
---
Only compressible text-oriented public assets receive deterministic `.zstd` sibling representations.

```yaml
eligible_media:
  - text/*
  - application/javascript
  - application/json
  - application/manifest+json
  - application/xml
  - image/svg+xml
typical_extensions:
  - .html
  - .css
  - .js
  - .mjs
  - .json
  - .map
  - .txt
  - .xml
  - .svg
  - .webmanifest
excluded:
  - images other than SVG
  - audio and video
  - archives, fonts, WebAssembly, and other binary formats
  - already compressed or encoded content
  - generated .zstd sidecars
rules:
  - determine eligibility from the original path and media type
  - a .zstd sidecar is an internal representation and never a public URL
  - source bytes remain present beside every sidecar
  - generated sidecars are build artifacts excluded from version control
```
