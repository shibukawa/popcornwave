---
id: decision:vscode-first-editor-target
type: decision
title: VS Code as the First Editor Target
---
Ship editor support for system:vscode first, and reach every other editor through api:cli-lsp rather than through a second extension effort.

```yaml
status: proposed
problem:
  - vision:editor-support has to start somewhere, and starting in three editors triples the packaging work before any of it is proven
  - a grammar written per editor drifts, because concept:template-source-dialects changes upstream in system:tinybind
decision:
  first: a system:vscode extension, per decision:extension-in-repository
  everything_after: decision:language-server-in-pw-cli, so the analysis half is editor-neutral from the moment it exists
reason:
  - the scaffolded project already carries a .vscode/settings.json, so the audience is already there
  - TextMate grammars are consumed unchanged by several other editors, so stage 1 is not thrown away by a second target
  - LSP is the only feature surface where a second editor costs a thin client instead of a reimplementation
consequences:
  - a JetBrains or Neovim user gets nothing at stage 1 and everything but highlighting at stage 2
  - the grammar is a duplicated definition of an upstream grammar and needs the drift guard rule:template-grammar-scopes states
  - no editor-specific behavior may live behind api:cli-lsp; a VS Code convenience belongs in the extension
rejected:
  tree_sitter_first: a tree-sitter grammar serves Neovim and Zed better, but system:vscode does not consume it, so it would be the second implementation rather than the first
  wait_for_the_server: highlighting is the whole of the request, needs no resolution, and would be blocked for a stage on work it does not depend on
```
