---
id: data:release-artifact
type: data
title: Release Artifact Set
---
One tag publishes one archive per supported target plus a single checksum file; every channel of requirement:cli-distribution addresses these names.

```yaml
tag: "v{major}.{minor}.{patch}" with an optional prerelease suffix
archive_name: "pw_{version}_{os}_{arch}.{ext}"
version_in_name: the tag without the leading v
ext:
  darwin: tar.gz
  linux: tar.gz
  windows: zip
contents:
  - pw executable, pw.exe on windows
  - README.md
  - LICENSE when the repository has one
  rule: no directory prefix; extraction yields the files directly
  open_issue: the repository declares no license, so no LICENSE file ships and neither channel states one
reproducibility:
  - archive member timestamps and ownership are fixed, not taken from the runner
  - gzip is invoked without a name or timestamp header
  - the same tag and toolchain rebuild to byte-identical archives
checksum_file:
  name: checksums.txt
  format: sha256sum lines over every archive of the same tag
  scope: archives only; the file never lists itself
build_flags:
  cgo: CGO_ENABLED=0
  trimpath: enabled
  ldflags:
    - -s -w
    - -X github.com/shibukawa/popcornwave/internal/pwcli.version={version}
  target: ./cmd/pw
publication:
  host: GitHub releases of the repository
  producer: decision:cli-release-pipeline
  consumers:
    - decision:homebrew-tap-channel formula urls
    - manual download
  not_consumed_by: decision:nix-flake-packaging, which builds from source
immutability:
  - a published archive is never replaced; a correction ships as a new patch tag
  - the checksum file is written once with the archives
  - a prerelease tag publishes a GitHub prerelease and never bumps the tap formula
```
