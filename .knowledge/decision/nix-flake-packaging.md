---
id: decision:nix-flake-packaging
type: decision
title: In-repository Nix Flake Packaging
---
The repository owns flake.nix and builds pw from source with buildGoModule; nixpkgs submission is deferred.

```yaml
status: accepted
flake:
  files: flake.nix and flake.lock, committed at the repository root
  inputs:
    nixpkgs: nixos-unstable, pinned by flake.lock, because it is the channel shipping the go toolchain required by go.mod
    flake-utils: rejected; genAttrs over an explicit system list needs no extra input
  outputs:
    packages.<system>.pw: the buildGoModule derivation
    packages.<system>.default: pw
    apps.<system>.default: the pw executable, so nix run works with no attribute path
    overlays.default: exposes pw to dependent flakes
    devShells.<system>.default: go, gopls, and the tools of decision:host-tools-target-runtime
    formatter.<system>: nixfmt
  systems: x86_64-linux, aarch64-linux, aarch64-darwin
  excluded_system:
    system: x86_64-darwin
    reason: nixpkgs 26.11 dropped it
    coverage: intel macOS is served by decision:homebrew-tap-channel and the data:release-artifact archive
    revisit: pinning the 26.05 darwin branch would restore it at the cost of an older toolchain channel
derivation:
  pname: pw
  version:
    released: a literal string in flake.nix equal to the tag without the leading v
    bump: written in the release preparation commit that precedes the tag, per flow:cli-release
    unreleased: the same string suffixed with the self short revision when building a non-tag checkout
  src: the repository, self, not a fetched archive
  vendorHash: recorded in flake.nix and updated whenever go.mod or go.sum changes
  subPackages: ./cmd/pw
  env.CGO_ENABLED: 0
  ldflags: the api:cli-version injection flags
  doCheck: go test of the pw command packages only; the full suite needs fixtures outside the sandbox
  installCheck: run pw version and match the derivation version
  meta: description, homepage, mainProgram pw, and license asl20
toolchain_risk:
  problem: go.mod requires a Go version that a given nixpkgs channel may not ship yet
  rule: the pinned nixpkgs input must provide it; otherwise override the go attribute of buildGoModule explicitly
  never: silently lower the go directive in go.mod to satisfy a channel
source_build_rationale:
  - a Nix user expects a reproducible source build over a repackaged binary
  - buildGoModule gives every supported system for free without a per-system archive
  - the archives of data:release-artifact stay the Homebrew and manual-download path
nixpkgs_deferred:
  reason: an upstream package makes every release wait on external review
  revisit_when: the tag cadence is stable and the flake has been proven across systems
  migration: the nixpkgs expression derives from the flake derivation; the flake stays authoritative
constraints:
  - the flake builds with no network access beyond the fixed-output vendor fetch
  - a vendorHash mismatch fails the build loudly and is fixed by updating the recorded hash
  - flake.lock is updated deliberately, never by an automated bump in this iteration
  - the flake exposes no application scaffolding; it packages the CLI only
verification:
  - nix build .#pw succeeds and its installCheck matches api:cli-version against the derivation version
  - nix run .#pw prints the same version line
  - nix flake check --all-systems passes with no warning
  - nix run github:shibukawa/popcornwave#pw stays unverified until the flake is pushed
```
