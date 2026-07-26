---
id: decision:cli-release-pipeline
type: decision
title: Hand-written CLI Release Pipeline
---
A hand-written GitHub Actions workflow cross-compiles, archives, and publishes data:release-artifact; no release-automation framework is adopted.

```yaml
status: accepted
workflow:
  file: .github/workflows/release.yml
  trigger: push of a tag matching v*
  manual: workflow_dispatch for a dry run that builds and smoke-tests without publishing
  permissions: contents write for the release job only
jobs:
  test:
    runner: ubuntu-latest
    step: go test ./... once, before any archive is produced
    reason: a matrix step would rerun the identical native suite per target
  build:
    needs: test
    runner: ubuntu-latest
    strategy: matrix over requirement:cli-distribution supported_targets, fail-fast
    steps:
      - checkout with full history so the tag is visible
      - setup-go with the go.mod toolchain version and module cache
      - resolve the version from the tag, or a dev string for a dispatch run
      - CGO_ENABLED=0 GOOS/GOARCH go build with the data:release-artifact flags
      - archive with fixed timestamps and ownership
      - smoke test the native archive only, because it is the one this runner can execute
      - upload as a workflow artifact
  publish:
    needs: build
    condition: the ref is a tag
    steps:
      - reject a tag whose release is already published
      - download every target archive
      - write checksums per policy:release-integrity
      - create a draft release carrying every asset, so a failed upload leaves a draft
      - record the Go toolchain version and the built commit in the notes
      - flip the draft to published
  tap:
    needs: publish
    condition: the version is not a prerelease
    step: render and push the formula bump described in decision:homebrew-tap-channel
alternatives_rejected:
  goreleaser: adds a config format and an external release framework for a five-target pure-Go binary and one tap formula
  manual_local_release: not reproducible, leaks the maintainer's toolchain version, and cannot be audited from the repository
  build_per_runner_os: unnecessary because the non-cgo build cross-compiles; a darwin runner would only slow the matrix
constraints:
  - the workflow never publishes when go test, a target build, or a smoke test fails
  - the workflow installs no tool implicitly; every action and toolchain version is pinned
  - no secret is exposed to the build job; only the tap job reads a token
  - the same command sequence is runnable locally for reproduction
  - a rebuilt tag produces byte-identical archives given the same toolchain version
verification:
  - a workflow_dispatch dry run produces every archive and publishes nothing
  - a tag push produces the release, the checksum file, and the tap bump exactly once
```
