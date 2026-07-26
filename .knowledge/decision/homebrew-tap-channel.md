---
id: decision:homebrew-tap-channel
type: decision
title: Homebrew Own-Tap Channel
---
Homebrew delivery starts in the maintainer-owned tap shibukawa/homebrew-tap installing prebuilt data:release-artifact binaries; homebrew-core is deferred.

```yaml
status: accepted
tap:
  repository: github.com/shibukawa/homebrew-tap
  formula_path: Formula/pw.rb
  install_command: brew install shibukawa/tap/pw
formula:
  class: Pw < Formula
  source: released archives, not a source build
  reason: the archive is already a static pure-Go executable, so a go build dependency buys nothing
  platform_selection: on_macos and on_linux with on_arm and on_intel url and sha256 pairs
  install: bin.install "pw"
  test: assert the api:cli-version output contains the formula version
  license and homepage: taken from the repository
  livecheck: repository tags matching v*
update:
  actor: the tap job of decision:cli-release-pipeline
  mechanism: checkout the tap, rewrite version, url, and sha256 values, commit, and push
  credential: a fine-grained token limited to the tap repository, stored as a repository secret
  idempotence: a bump for an already-current version is a no-op, not a failure
  fallback: the formula is editable by hand; the release does not fail when the tap push fails, but it reports the failure
homebrew_core_deferred:
  reason: core requires notability and a stable release history that does not exist yet
  revisit_when:
    - the tag series has run long enough to show a stable cadence
    - the project meets the core notability guideline
  migration: core adoption keeps the tap as a redirect period, and both cannot publish the same formula name silently
constraints:
  - the tap holds only formula files; nothing in it is generated from .knowledge
  - the formula never downloads anything beyond the pinned archive urls
  - a checksum mismatch is a formula bug and fails the install, per policy:release-integrity
  - windows is out of scope for this channel
```
