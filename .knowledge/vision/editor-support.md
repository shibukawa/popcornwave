---
id: vision:editor-support
type: vision
title: Editor Support Vision
---
Popcorn Web ships editor support for its own source dialects, so a .pw.html, .pw.sql, or .pw.dynamo file reads and edits as a known language instead of as plain text.

```yaml
status: stages 1, 1.5, and 2 shipped; stage 3 shipped except for the browser-report source of requirement:editor-runtime-diagnostics, the web build of requirement:editor-web-host, and the four resolver-bound jumps requirement:editor-navigation still lists
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
    - requirement:editor-workspace-symbols
    ships: api:cli-lsp, so the analysis is Go and reusable, per decision:language-server-in-pw-cli
    delivered: complete. api:cli-lsp serves syntax and placement diagnostics, documentSymbol, and workspace/symbol over a loaded project model; the extension starts it under policy:editor-tool-execution and reaches the CLI commands as tasks. What the editor still cannot report is a finding needing a resolved type, which is stage 3's own dependency
  3:
    - requirement:editor-navigation
    - requirement:editor-completion
    - requirement:editor-inlay-hints
    - requirement:editor-generated-peek
    - requirement:editor-route-explorer
    - requirement:editor-story-preview
    - requirement:editor-runtime-diagnostics
    - requirement:editor-web-host
    - requirement:additional-editor-clients
    ships: the features that need resolved types rather than only a parse tree, and the surfaces that make the framework's own indirection visible
    delivered: the type graph and every LSP capability above it, plus the three pw/ methods LSP has no equivalent for. What is not delivered is named in each requirement's own status rather than summarized here, because each is missing a different input
  4:
    - requirement:editor-code-actions
    - requirement:declaration-rename
    ships: the features that write; they are last because each edits source the developer did not open
outside_the_staging:
  requirement:template-source-positions: >
    a generator property rather than an editor feature. requirement:editor-navigation,
    requirement:editor-generated-peek, and requirement:editor-runtime-diagnostics all resolve
    through it, and the Go toolchain gains it with no editor running, so it does not wait for a stage
principles:
  - the editor never reimplements a parser; it runs upstream's own code, through api:cli-lsp or through the WebAssembly build of decision:formatter-delivery
  - a stage ships alone and is useful alone, so stage 1 needs no binary and no configuration
  - what the editor reports and what api:cli-generate reports are the same message, per decision:shared-check-catalog
  - generated artifacts are output, not source; policy:generated-artifacts decides what the editor offers to edit
  - the extension runs project tools under policy:editor-tool-execution and downloads none
non_goals:
  - a TypeScript reimplementation of the tinybind template parsers
  - Go language support, which gopls already owns
  - a debugger, a profiler, or a telemetry surface of Popcorn Web's own; requirement:dev-telemetry-viewer stays a browser tool, and requirement:template-source-positions makes Go's existing debugger name the template rather than adding one
  - editor-only diagnostics that api:cli-doctor and api:cli-generate cannot also produce
  - code actions before stage 4, and any layout rule of Popcorn Web's own; requirement:template-formatting keeps the layout upstream
placement: decision:extension-in-repository
```
