---
id: decision:extension-in-repository
type: decision
title: Extension Lives in the Framework Repository
---
The system:vscode extension sources live in this repository under tools/vscode, so a change to concept:template-source-dialects and the grammar that highlights it land in one commit.

```yaml
status: proposed
location: tools/vscode
problem:
  - the grammar restates upstream syntax, so a separate repository makes drift invisible until a user reports it
  - a separate repository needs its own release coordination with requirement:cli-distribution, and api:cli-lsp ships in the CLI
decision:
  sources: tools/vscode, holding the extension manifest, the grammars, and the language configuration
  fixtures: the grammar test corpus reuses the repository's existing .pw.html, .pw.sql, and .pw.dynamo sources rather than copies
  build: a Node toolchain confined to tools/vscode, invoked only by requirement:extension-distribution and its CI job
  go_build_isolation: the extension's Go source is only the decision:formatter-delivery WebAssembly entry, and it is a Go module of its own, so go build ./... and the TinyGo matrix never reach it; a build tag was the plan and a nested module turned out to be both simpler and stricter
  wasm_artifact: the compiled module is committed, so an install and npm test need no Go toolchain, and CI rebuilds it to verify the commit matches
  independent_pin: the nested module pins its own tinybind version, which is what lets the extension ship a formatter the framework has not adopted yet
consequences:
  - the repository gains a Node dependency tree that no Go build, no api:cli-build, and no scaffolded project touches
  - and, from decision:formatter-delivery, a TinyGo build step whose output is committed beside the grammars
  - a grammar test runs in CI beside go test, and a syntax change that breaks it fails the same pull request
  - the extension version and the CLI version are related but not equal; requirement:extension-distribution owns the pairing rule
  - concept:project-layout is unaffected, because it describes a generated application and not this repository
rejected:
  separate_repository: cheaper Node isolation, paid for with invisible drift and a second release calendar
  tinybind_go: the grammar's owner by syntax, but the extension is branded Popcorn Wave, ships api:cli-lsp behavior, and reads data:project-config, none of which upstream has
```
