---
id: requirement:pw-language-server
type: requirement
title: Popcorn Web Language Server
---
One Go language server, reached through api:cli-lsp, answers every editor feature past highlighting by reusing the parsers, the project loader, and the check catalog the CLI already runs.

```yaml
status: proposed
stage: 2 of vision:editor-support
mechanism: decision:language-server-in-pw-cli
surface: api:cli-lsp
documents:
  kinds: the three concept:template-source-dialects suffixes, plus popcornweb.toml for requirement:editor-diagnostics
  go_sources: read for resolution, never served; gopls owns Go documents
  generated: a *_pw_gen.go is an input to navigation and never a document the server offers to change
project_model:
  root: the nearest popcornweb.toml, per data:project-config
  purposes: decision:explicit-generation-sources decides which directory contributes which source kind, so the server answers the same scope api:cli-generate reads
  no_project: parse-only mode; syntax diagnostics still work and every resolved feature reports unavailable rather than guessing
  reload: a popcornweb.toml change reloads the project model without restarting the process
capabilities:
  stage_2:
    - textDocument/publishDiagnostics, per requirement:editor-diagnostics
    - textDocument/documentSymbol, listing declarations of the open file
    - textDocument/semanticTokens, refining rule:template-grammar-scopes rather than replacing it
  stage_3:
    - textDocument/definition, references, and hover, per requirement:editor-navigation
    - textDocument/completion, per requirement:editor-completion
  deferred:
    - rename, because a declaration name decides a generated Go symbol and a cross-file rename needs the generator's own view
    - formatting, because no canonical formatter for the dialects exists yet
performance:
  budget: a keystroke reparse of one document stays interactive; a whole-project analysis runs on save and on project load
  incremental: parse the open document only; resolution reuses the cached project model
  no_generation: the server never writes generated output, per policy:editor-tool-execution
lifecycle:
  start: on the first document of a registered language in a trusted workspace
  stop: with the window, or on an explicit restart command
  crash: the client restarts a bounded number of times and then falls back to stage-1 behavior with a single notification
acceptance:
  - a syntax error in an open document is reported without saving
  - a file opened outside any project still reports syntax diagnostics and reports no project-scoped diagnostic
  - a popcornweb.toml edit changing a generate purpose changes which files report the out-of-purpose diagnostic, with no restart
  - the server writes no file in the workspace during an editing session
  - semantic tokens and grammar scopes never disagree on the same range
```
