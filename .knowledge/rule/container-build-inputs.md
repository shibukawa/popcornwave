---
id: rule:container-build-inputs
type: rule
title: Container Build Inputs
---
A container build of a Popcorn Wave application runs the decision:host-tools-target-runtime host phase inside the image before any compiler reads the sources, because the files that phase writes are the ones policy:generated-artifacts keeps out of version control.

```yaml
why_it_is_a_rule: the Dockerfile every Go reader already knows how to write is COPY . . followed by go build, and that build fails on a Popcorn Wave project for reasons the compiler diagnostic does not name
missing_without_the_host_phase:
  generated_go: "{source-base}_pw_gen.go beside every .pw.html, .pw.sql, and registered handler, excluded by the api:cli-init .gitignore rule"
  bootstrap: cmd/{name}/popcornwave_bootstrap_pw_gen.go, without which no registration package is linked
  embedded_asset_tree: dist/public, which public.go names in a go:embed directive, so its absence is a compile error rather than an empty tree
  generated_css: public/generated/app.css for a project with requirement:tailwind-css-integration
required_order:
  - api:cli-generate
  - flow:tailwind-css-build in production mode, only when Tailwind is enabled
  - flow:public-asset-build
  - the compiler for data:project-config project.toolchain
single_command:
  host_go: api:cli-build performs all four
  any_other_compiler: api:cli-prepare performs the first three and stops, per decision:explicit-tinygo-compile-step, so the caller supplies only the last one
tools_the_builder_stage_needs:
  pw: system:pw-cli, installed with go install because requirement:cli-distribution publishes no container image
  pinned: the pw version is written by api:cli-init and must track the framework version in go.mod, since api:cli-generate output is read by the framework it was generated against
  tailwind: the decision:tailwind-host-toolchain standalone executable, only when Tailwind is enabled; Devbox does not reach inside an image, so the pinned version is repeated in the Dockerfile
  go: present for a TinyGo build too, because the host phase is host Go whatever compiles the application
consequences:
  dockerignore: a host copy of the generated Go or of dist is excluded, because the image rebuilds both and a stale copy would win
  dist_sentinel: excluding dist removes dist/public/.keep, so flow:public-asset-build must create the directory before the compiler reads the embed directive
  cache: COPY go.mod go.sum and go mod download before COPY . . , because the host phase and the compiler share the module cache
  builders_that_only_run_go_build: requirement:dockerless-image-builders records what ko and Cloud Native Buildpacks do with this rule
verification: a build from a clean git clone with no host toolchain produces a running image
```
