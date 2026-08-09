---
id: flow:derived-asset-build
type: flow
title: Derived Asset Build Flow
---
One host phase turns the authored tree and the generation-time conversions into concept:derived-public-tree, its sidecars, and data:public-asset-manifest, before Go embeds any of it.

```yaml
trigger:
  - api:cli-build
  - api:cli-dev, per decision:derived-tree-development
replaces: flow:public-asset-build, whose sidecar step becomes one step of this one
inputs:
  authored: project-root public, walked without following symbolic links
  generated_css: flow:tailwind-css-build output, which enters as an ordinary input rather than as a special case
  produced: the files api:cli-generate reference hooks declared, per requirement:derived-asset-pipeline
steps:
  - run api:cli-generate first, since a hook produces files and rewrites references while templates compile
  - classify every authored file per policy:asset-transform-matrix
  - copy or transform each into dist/public, preserving its relative path
  - place every produced file into the same tree and fail on a name two producers claim
  - drop the authored bytes of any source whose conversion succeeded
  - write .br, .zstd, and .gz sidecars for eligible results, per policy:public-asset-precompression
  - digest every emitted representation and write data:public-asset-manifest
  - verify the scaffolded public.go embeds dist/public and that the tree is non-empty
  - compile only after the tree and the manifest are complete
ordering_note:
  why_generate_first: a reference rewrite decides a URL, and the manifest cannot be written before every URL exists
  why_sidecars_last: a sidecar is a representation of a final byte sequence, so compressing before conversion would compress bytes nobody serves
reconciliation:
  form: the tree is derived, so a removed source removes its outputs by construction rather than by a cleanup rule
  stale: an output whose source and producer both disappeared cannot survive a rebuild, which is the cleanup requirement:component-asset-extraction still lacks in the authored tree
failure:
  - preserve the previous complete tree and manifest
  - never embed a tree and a manifest from different runs
  - a conversion error names the source, and a hook error names the template file, line, and column
reproducibility:
  - identical inputs, settings, and toolchain versions produce identical bytes, names, and manifest order
  - toolchain identity is hashed per decision:asset-transform-toolchain
empty_tree: a project with no asset still emits a sentinel and a manifest, since go:embed fails on an absent directory
```
