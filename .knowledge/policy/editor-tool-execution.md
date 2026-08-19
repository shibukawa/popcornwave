---
id: policy:editor-tool-execution
type: policy
title: Editor Tool Execution Safety
---
Opening a file must never run project code; the extension runs only a resolved pw binary, only for an explicitly enabled workspace, and never reaches the network.

```yaml
status: proposed
scope: the system:vscode extension and every process it starts
embedded_wasm:
  what: the decision:formatter-delivery WebAssembly build of the upstream formatter, shipped inside the vsix
  status: not a binary this policy restricts; it is extension content, signed and versioned with the package
  reach: instantiated with no filesystem preopen, no network capability, and no argument taken from workspace content beyond the buffer being formatted
  untrusted_workspace: allowed, because it starts no process and reads nothing the editor has not already opened
binary_resolution:
  order: the workspace Devbox environment, then PATH, then a user-configured absolute path
  never: download, install, or update a binary, in any stage
  missing: report once with the install instruction of requirement:cli-distribution, and stay in stage-1 behavior
  version: report a mismatch between the resolved pw and the project's expectation; never resolve it silently
workspace_trust:
  untrusted: the grammar loads, the embedded formatter runs, and no process starts
  restricted_mode: the extension declares limited support and disables every requirement:editor-tasks command and api:cli-lsp
  reason: a workspace-relative binary path and a Devbox environment are both workspace-controlled inputs
process_rules:
  - api:cli-lsp runs read-only and writes no file in the workspace
  - a command that writes, such as api:cli-generate, runs only from an explicit user action, never from a save or an open
  - api:cli-migrate and api:cli-dev are never started implicitly, because both change state outside the editor
  - every process is a child of the extension host and stops with the window
  - a command runs in a visible terminal or an output channel, so what ran is inspectable
network:
  extension: contacts nothing, with one stated exception below
  loopback_exception:
    what: requirement:editor-runtime-diagnostics reads the requirement:dev-console loop state over loopback
    bounds: off by default and contacting nothing until switched on; one host, which is the console port data:project-config declares; a read, never a write; and the record itself never leaves the machine
    why_it_is_allowed: the console is a process the developer started on their own machine, and the alternative is a class of failure the editor cannot see at all
    why_it_is_stated: an exception a reader has to infer from an implementation is one that grows; this is the only one, and a second needs its own line here
  server: api:cli-lsp runs with the api:cli-doctor online check set disabled, so no configured endpoint is probed from a keystroke
data:
  - no source, no configuration value, and no diagnostic leaves the machine
  - a value redacted by policy:startup-summary or policy:query-log-safety stays redacted in every editor surface
```
