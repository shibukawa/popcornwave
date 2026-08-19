---
id: api:cli-lsp
type: api
title: pw lsp
---
pw lsp speaks the Language Server Protocol over stdio for one workspace, exposing the same analysis api:cli-generate and api:cli-doctor already run.

```yaml
status: stdio transport, the data:project-config model, syntax diagnostics, documentSymbol, and workspace/symbol implemented at internal/pwlsp; the capabilities needing a resolved type are unimplemented
usage: "pw lsp [--stdio] [--log=path] [--root=path]"
requirement: requirement:pw-language-server
mechanism: decision:language-server-in-pw-cli
transport:
  default: stdio, the only mode the first release ships
  reason: a socket mode adds an authentication question that no editor client here needs
inputs:
  root: the workspace root; the project is the nearest data:project-config below or at it
  log: a file for protocol and diagnostic tracing, off by default
analysis_reuse:
  parsers: the system:tinybind template parsers, unchanged, so a diagnostic position matches a generation error position
  project: the data:project-config loader, so an unknown key is the same error in both commands
  checks: the decision:shared-check-catalog runner, restricted to checks whose inputs an editor can build
  dialect: data:project-config project.database selects the SQL dialect, exactly as flow:sql-generation does
boundaries:
  writes: none; the command is read-only, per policy:editor-tool-execution
  network: none; the api:cli-doctor online check set is disabled
  generation: never; an editor that wants generated output runs api:cli-generate through requirement:editor-tasks
  environment: analysis runs for one data:runtime-environment token, defaulting to dev, and never injects one into a process
failure:
  no_project: serve syntax-only analysis and report the absent project once
  unreadable_project: report the load error as a workspace diagnostic and keep serving syntax analysis
  panic: exit nonzero with the protocol closed cleanly, so the client's restart policy applies
distribution: shipped in the same binary as every other command, per requirement:cli-distribution and flow:cli-release
```
