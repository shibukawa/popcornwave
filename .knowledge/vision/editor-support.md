---
id: vision:editor-support
type: vision
title: Editor Support Vision
---
Popcorn Web ships editor support for its own source dialects, so a .pw.html, .pw.sql, or .pw.dynamo file reads and edits as a known language instead of as plain text.

```yaml
status: proposed
problem: concept:template-source-dialects files open with no grammar, so a declaration header, an embedded expression, and the HTML or SQL body all render as one undifferentiated block
primary_actor: actor:application-developer
first_target: system:vscode, per decision:vscode-first-editor-target
staging:
  1:
    - requirement:editor-language-registration
    - requirement:template-syntax-highlighting
    - requirement:extension-distribution
    ships: an extension with no process of its own, per decision:textmate-grammar-first
  1.5:
    - requirement:editor-formatting
    ships: the system:tinybind formatter, embedded as WebAssembly and delegated to api:cli-fmt where it can be, per decision:formatter-delivery
    why_not_stage_2: formatting reads one buffer and no project model, so it does not wait for requirement:pw-language-server
  2:
    - requirement:pw-language-server
    - requirement:editor-diagnostics
    - requirement:editor-tasks
    ships: api:cli-lsp, so the analysis is Go and reusable, per decision:language-server-in-pw-cli
  3:
    - requirement:editor-navigation
    - requirement:editor-completion
    ships: the features that need resolved types rather than only a parse tree
principles:
  - the editor never reimplements a parser; it runs upstream's own code, through api:cli-lsp or through the WebAssembly build of decision:formatter-delivery
  - a stage ships alone and is useful alone, so stage 1 needs no binary and no configuration
  - what the editor reports and what api:cli-generate reports are the same message, per decision:shared-check-catalog
  - generated artifacts are output, not source; policy:generated-artifacts decides what the editor offers to edit
  - the extension runs project tools under policy:editor-tool-execution and downloads none
non_goals:
  - a TypeScript reimplementation of the tinybind template parsers
  - Go language support, which gopls already owns
  - a debugger, a profiler, or a telemetry surface; requirement:dev-telemetry-viewer stays a browser tool
  - editor-only diagnostics that api:cli-doctor and api:cli-generate cannot also produce
  - code actions before stage 3, and any layout rule of Popcorn Web's own; requirement:template-formatting keeps the layout upstream
placement: decision:extension-in-repository
```
