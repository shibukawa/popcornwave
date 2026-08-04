---
id: decision:language-server-in-pw-cli
type: decision
title: Language Server Inside the pw CLI
---
Editor analysis is a system:pw-cli subcommand written in Go, not a TypeScript service inside the extension, because the parsers, the check catalog, and the project loader already exist there.

```yaml
status: proposed
problem:
  - requirement:editor-diagnostics must report exactly what api:cli-generate reports, and a second implementation of the same parse diverges
  - decision:shared-check-catalog already refused a second implementation of one condition for api:cli-doctor
  - an extension-local analyzer would be the only consumer of a TypeScript port of concept:template-source-dialects
decision:
  surface: api:cli-lsp
  language: Go, built and released with the CLI, per decision:host-tools-target-runtime
  reuse: the system:tinybind template parsers, the data:project-config loader, and the decision:shared-check-catalog runner
  client: the system:vscode extension is a thin language client and holds no analysis
alternatives_rejected:
  typescript_analyzer: needs a port of the parsers and drifts on every tinybind release
  shell_out_to_pw_generate: usable for a save-triggered check and too slow for keystrokes; requirement:editor-diagnostics keeps it only as the stage-2 fallback
  separate_lsp_binary: a second artifact in flow:cli-release and a second version to reconcile with the project's pinned CLI
consequences:
  - the editor needs pw on PATH or in the Devbox environment, which policy:editor-tool-execution bounds
  - a project pinning an older pw gets that version's analysis, which matches what its api:cli-generate would do and is the correct answer
  - JetBrains and Neovim reach every stage-2 and stage-3 feature with a client and no analysis, per decision:vscode-first-editor-target
  - api:cli-lsp joins the system:pw-cli command list and inherits requirement:cli-distribution
  - a check whose inputs the editor cannot build is skipped and reported as skipped, exactly as decision:shared-check-catalog already requires of api:cli-doctor
```
