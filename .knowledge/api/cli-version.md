---
id: api:cli-version
type: api
title: pw version
---
pw version prints the version of the installed binary so every channel of requirement:cli-distribution has one verifiable install check.

```yaml
usage: pw version
also_accepts:
  - pw --version
  - pw -v
output:
  line: "pw {version} ({commit}, {os}/{arch}, go{toolchain})"
  stream: stdout
  exit: 0
resolution_order:
  - the value injected by the data:release-artifact ldflags
  - runtime/debug.ReadBuildInfo Main.Version for a plain go install
  - the vcs.revision and vcs.modified build settings for a local build
  - the literal devel when nothing is available
implementation:
  variable: an unexported package-level string in internal/pwcli set through -X
  rule: the variable is written only by the linker; no build step rewrites source
constraints:
  - the command opens no file, reads no configuration, and needs no project root
  - the output line is stable enough for the formula and derivation install checks to match on it
  - the version string never contains a leading v when it came from a tag
usage_in_packaging:
  homebrew: the formula test block asserts the version substring
  nix: the installCheck phase asserts the derivation version
```
