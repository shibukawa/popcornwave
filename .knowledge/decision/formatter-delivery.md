---
id: decision:formatter-delivery
type: decision
title: Formatter Delivery to the Editor
---
Ship the system:tinybind formatter to the editor twice: as a WebAssembly module inside the extension, which always works, and as api:cli-fmt, which is preferred whenever the project has one.

```yaml
status: embedded path implemented at tools/vscode 0.3.0, carrying system:tinybind v0.5.16; delegated path waits on api:cli-fmt
problem:
  - requirement:editor-formatting needs the upstream Go formatter to run inside a system:vscode extension host
  - decision:textmate-grammar-first bought stage 1 a property worth keeping: highlighting works with no binary, no project, and no trust
  - a formatter that only works with a resolved pw binary loses that property for the one feature a reader most expects to just work
  - a formatter that only ships in the extension can format differently from the pw the project pins, which is worse than not formatting
measurement:
  taken: 2026-08-02, against tinybind-go v0.3.1, formatting every .pw source in this repository
  tinygo_wasip1: 608 KB, 233 KB compressed into the vsix, correct output on every source that parses; 610 KB once the pin moved to v0.3.2, 672 KB at v0.5.16
  go_wasip1: 3.7 MB stripped, 1.05 MB compressed, same output
  latency: about 50 ms for a whole cold process including instantiation, so an in-process cached module is far under a save
  measured_again_after_implementing: 15 ms for the first format including compilation, then a 2.4 ms median for a guarded format, which is two full passes over a real component
  packaged: the vsix grew from 19 KB to 252 KB, and to 280 KB at v0.5.16
  conclusion: TinyGo is the target, which also keeps the extension honest with decision:force-tinygo-logic
decision:
  embedded:
    build: templatefmt behind a small wasip1 entry, compiled with TinyGo, committed as a build artifact of the extension
    isolation: the entry is a Go module of its own under the extension, which excludes it from the root go build ./... more simply than a build tag would
    host_interface: the seven WASI preview1 functions the module imports, shimmed in the extension in about 150 lines rather than taken from a package, because the extension host's Node may not expose node:wasi and a web host has none
    reproducibility: CI rebuilds the module and fails when the committed one formats any of the repository's own sources differently, pinned to the TinyGo version the extension records
    why_not_a_hash: TinyGo 0.41.1 emits 610613 bytes on darwin/arm64 and 614645 on linux/amd64 from the same source, the same Go and the same Binaryen, and the Go patch level moves it again, so a hash committed from a maintainer's machine is one the Linux runner can never reproduce; the output is what the extension promises and it is identical across those builds
    call_contract: a dialect argument, the source on stdin, the result on stdout, which is the same shape api:cli-fmt --stdin will have, so the delegated path is a swap rather than a rewrite
    used_when: no project, no resolved pw, an untrusted workspace, or a pw too old to have api:cli-fmt
    reach: pure functions over a byte slice, so the module needs no filesystem preopen and no network capability
  delegated:
    used_when: a trusted workspace with a resolved pw that has api:cli-fmt
    why_preferred: it is the version the project pins, so the editor and CI agree by construction
    transport: the api:cli-fmt --stdin filter mode, because an editor formats a buffer rather than a file
    detection: run pw fmt --stdin against an empty buffer once per session; a nonzero exit or an unknown-command error selects the embedded path
    guard_floor: a project pinning below system:tinybind v0.3.2 has no idempotence guard, so this path either refuses that version or restores the check requirement:editor-formatting dropped
    now_possible: api:cli-fmt exists and the framework is on v0.3.5, so this path is unblocked and simply not written yet
  reporting: the extension names which of the two produced a result the first time it formats in a session, so a surprising diff is traceable
why_not_only_the_cli:
  - a .pw.html opened from a review checkout has no project and no binary, and refusing to format it is the behavior stage 1 deliberately avoided
  - policy:editor-tool-execution disables every spawned process in an untrusted workspace, which would take formatting with it
why_not_only_wasm:
  - the extension version and the project's pinned tinybind version are independent, so the editor could canonicalize a file that CI then rejects
  - the delegated path removes that class of disagreement wherever it is available
why_wasm_is_not_a_downloaded_binary:
  - the module ships inside the signed vsix and is versioned with it, so policy:editor-tool-execution's no-download rule is satisfied rather than waived
  - it runs in the extension host sandbox with no filesystem and no network, which is strictly less reach than a spawned pw
consequences:
  - the extension gains a build step, a Go and TinyGo toolchain in its CI job, and a committed wasm artifact, all new to decision:extension-in-repository
  - the vsix grows from about 19 KB to a few hundred, which is unremarkable for the category and worth naming anyway
  - two paths mean two behaviors to keep honest; the reporting line above is what makes a difference visible instead of mysterious
  - the wasm module is a second copy of upstream analysis, which decision:language-server-in-pw-cli refused for diagnostics; the difference is that this is the same compiled Go rather than a reimplementation, and that formatting has no project-wide inputs to disagree about
  - requirement:pw-language-server may later serve textDocument/formatting over the delegated path, which changes the transport and neither of these two answers
rejected:
  javascript_port: a second layout implementation, drifting on every upstream release, for the one feature where a byte difference is the whole output
  bundle_the_pw_binary: platform matrix in the vsix, and still the wrong version for a project that pins another
  defer_until_the_language_server: formatting needs no project model, so making it wait on one trades a year for nothing
```
