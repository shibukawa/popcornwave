---
id: policy:release-integrity
type: policy
title: Release Integrity
---
Every installed pw binary must be traceable to one tag, one build recipe, and one published checksum.

```yaml
versioning:
  scheme: semantic versioning on tags prefixed with v
  pre_1_0: minor bumps may break the CLI surface; the release notes state it
  authority: the tag is the only version source; no version file is committed
  reporting: api:cli-version reports what the binary was built as, never what the repository currently is
provenance:
  - only decision:cli-release-pipeline publishes artifacts
  - artifacts are built from a tagged commit of this repository, never from a local machine
  - the build records the Go toolchain version in the release notes
  - trimpath removes local paths from the binary
integrity:
  - every archive has a sha256 entry in the data:release-artifact checksum file
  - the decision:homebrew-tap-channel formula pins the sha256 of each archive it downloads
  - decision:nix-flake-packaging pins sources through flake.lock and vendorHash
  - a checksum mismatch aborts the install; it is never warned past
secrets:
  - the tap token is scoped to the tap repository and to contents write
  - no secret is available to a build or test job
  - a pull request from a fork never reaches a job that holds a secret
  - a released binary embeds no token, path, or environment value
supply_chain:
  - actions are pinned by commit sha with the readable version in a trailing comment, and upgraded deliberately
  - a mutable major tag is not a pin; the release workflow handles secrets and publishes artifacts
  - the release build resolves modules from the module cache of the pinned toolchain and adds no new dependency
  - CGO_ENABLED=0 keeps the artifact free of host C libraries per decision:sqlite-backend-selection
deprecation:
  - a yanked release is marked on GitHub and superseded by a patch tag; artifacts stay downloadable
  - the tap formula is moved forward, not rewritten to point at a deleted artifact
```
