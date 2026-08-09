---
id: policy:public-asset-precompression
type: policy
title: Public Asset Precompression
---
Only compressible text-oriented public assets receive deterministic sibling representations, one per coding of decision:response-content-codings that a build produces.

```yaml
codings:
  brotli:
    suffix: .br
    level: maximum
    only_here: a served binary links no brotli encoder, so this is the sole place the coding exists
  zstd:
    suffix: .zstd
    level: maximum
  gzip:
    suffix: .gz
    level: maximum
  why_all_three: build CPU is not request latency, so each coding is encoded at the level it is worth, and the ratios separate far enough to matter only at those levels
  measured_spread: brotli beats zstd by roughly 15 percent and gzip by roughly 17 percent at maximum levels, per decision:response-content-codings
encoders:
  brotli: github.com/andybalholm/brotli, reachable from api:cli-build and from nothing a deployment runs
  zstd: requirement:contrib-zstd host implementation
  gzip: the host backend of requirement:response-gzip-encoder
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
  - generated sidecars of any coding
rules:
  - determine eligibility from the original path and media type, identically for every coding
  - a sidecar is an internal representation and never a public URL
  - source bytes remain present beside every sidecar
  - generated sidecars are build artifacts excluded from version control
  - a coding whose result is not smaller than the source is skipped for that file with a reason, per the skip shape of policy:asset-transform-matrix
  - a skipped or missing coding is not an error; policy:public-asset-negotiation falls through to the next one
  - eligibility is per file and not per coding, so a file either has every producible sidecar or none
```
