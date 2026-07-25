---
id: requirement:cli-distribution
type: requirement
title: pw CLI Distribution
---
system:pw-cli is installable without a Go toolchain through a Nix flake and a Homebrew tap, both fed by one tagged GitHub release.

```yaml
motivation: decision:host-tools-target-runtime makes pw a prerequisite of every project, so installation must precede any Go setup
channels:
  nix: decision:nix-flake-packaging
  homebrew: decision:homebrew-tap-channel
  go_install: retained as the toolchain-native path, never the documented default
single_source:
  tag: one semantic version tag drives every channel
  artifacts: data:release-artifact
  pipeline: decision:cli-release-pipeline
  flow: flow:cli-release
version_surface: api:cli-version
integrity: policy:release-integrity
supported_targets:
  - darwin/arm64
  - darwin/amd64
  - linux/amd64
  - linux/arm64
  - windows/amd64
build_mode:
  cgo: disabled
  sqlite_backend: modernc.org/sqlite selected by decision:sqlite-backend-selection non-cgo constraint
  consequence: every target cross-compiles from one linux runner with no C toolchain
acceptance:
  - nix run github:shibukawa/popcornwave#pw prints usage on a clean machine
  - nix profile install github:shibukawa/popcornwave installs pw into the profile
  - brew install shibukawa/tap/pw installs pw on darwin arm64 and amd64
  - both channels report the released tag through api:cli-version
  - a released archive extracts exactly the data:release-artifact contents with no directory prefix
  - checksums published with the release verify every archive
  - api:cli-migrate and api:cli-init succeed from an installed binary with no Go toolchain present
  - the release job fails, and publishes nothing, when any target fails to build or any smoke test fails
  - re-running the release for an existing tag does not overwrite published artifacts
non_goals:
  - homebrew-core submission in this iteration; see decision:homebrew-tap-channel
  - nixpkgs submission in this iteration; see decision:nix-flake-packaging
  - Linux distribution packages, container images, and Windows package managers
  - a curl-to-shell install script
  - self-update inside pw
```
