---
id: requirement:additional-editor-clients
type: requirement
title: Editors Beyond VS Code
---
A second editor reaches stage 2 and stage 3 by starting api:cli-lsp with the configuration this repository publishes, and reaches stage 1 through a grammar it can consume, so no editor requires a second analysis.

```yaml
status: the published configuration is implemented as documentation for Neovim and Zed; no client package is released for either
stage: 3 of vision:editor-support
follows: decision:vscode-first-editor-target, which chose the word first and left the second target unnamed
second_target:
  editor: Zed
  why: it consumes a language server through a small extension manifest, and its highlighting wants a tree-sitter grammar rather than the TextMate one decision:vscode-first-editor-target ships
  cost: the language-server half is configuration; the highlighting half is the second grammar that decision rejected as a first target
highlighting_by_editor:
  textmate_consumers: reuse the rule:template-grammar-scopes grammars unchanged
  tree_sitter_consumers: need a grammar of their own, which is a second definition of an upstream syntax and needs the drift guard rule:template-grammar-scopes states, run against the same corpus
  alternative: rely on requirement:pw-language-server semantic tokens alone, which highlights only with a running server and therefore drops the property decision:textmate-grammar-first exists to keep
published_per_editor:
  configuration: the command, the file types, and the root marker popcornweb.toml, so a user configures nothing by hand
  packaging: the smallest artifact the editor's registry accepts, and no vendored binary, per policy:editor-tool-execution
  version_floor: the minimum pw version each feature needs, the same statement requirement:extension-distribution makes for the vsix
constraints:
  no_editor_specific_server_behavior: decision:vscode-first-editor-target already forbids it; a convenience belongs in that editor's client
  binary_resolution: each client resolves pw by the rules of policy:editor-tool-execution and downloads nothing
non_goals:
  - a JetBrains plugin, which is a plugin platform rather than a client and is its own project
  - shipping a client before requirement:pw-language-server exists, since a client with no server is a grammar package
  - a second formatter delivery; a non-VS-Code client uses api:cli-fmt, per decision:formatter-delivery
acceptance:
  - a Zed user with pw on PATH gets diagnostics from the published configuration alone
  - the tree-sitter grammar, if written, is checked against the same corpus rule:template-grammar-scopes uses
  - no client carries analysis code
  - an editor with no client still works by pointing a generic LSP configuration at api:cli-lsp
```
