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
  build:
    runner: ubuntu-latest
    strategy: matrix over requirement:cli-distribution supported_targets
    steps:
      - checkout with full history so the tag is visible
      - setup-go with the go.mod toolchain version and module cache
      - go test ./... once on the native target before any archive is produced
      - CGO_ENABLED=0 GOOS/GOARCH go build with the api:cli-version ldflags
      - archive and upload as a workflow artifact
  publish:
    needs: build
    steps:
      - download every target archive
      - write and sign off checksums per policy:release-integrity
      - create the GitHub release from the tag with generated notes
      - upload archives and the checksum file
  tap:
    needs: publish
    step: dispatch the formula bump described in decision:homebrew-tap-channel
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
