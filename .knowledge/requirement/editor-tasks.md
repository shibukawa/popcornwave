---
id: requirement:editor-tasks
type: requirement
title: Editor Commands and Tasks
---
The system:pw-cli commands a developer runs while editing are reachable from the editor, with their output parsed into the Problems view instead of only printed.

```yaml
status: proposed
stage: 2 of vision:editor-support
safety: policy:editor-tool-execution
commands:
  generate:
    runs: api:cli-generate --code-only
    trigger: explicit only, because it writes files
    output: parsed by a problem matcher into file, line, and message
    why_the_flag: an editor command writes the generated Go a diagnostic points into; the asset tree and the minified stylesheet the unflagged command also builds are a build's concern, not a keystroke's
  check:
    runs: api:cli-check
    use: a drift check that writes nothing, safe to bind to a save-time task
  dev:
    runs: api:cli-dev
    terminal: a dedicated long-lived terminal, because the loop owns services, the identity provider, and the telemetry viewer
    single_instance: a second invocation focuses the running terminal rather than starting a second loop
    never_implicit: the loop starts processes outside the editor
  doctor:
    runs: api:cli-doctor --format=json
    output: rendered as diagnostics on the files and configuration lines the report names
  migrate:
    runs: api:cli-migrate
    confirmation: required in the editor, because policy:migration-safety makes it forward-only against a real database
  restart_server:
    runs: nothing; restarts the api:cli-lsp client
task_provider:
  contributes: the same commands as resolvable tasks, so a project can compose them in tasks.json
  problem_matchers: contributed by the extension so a project writes none
integration:
  telemetry_viewer: the api:cli-dev banner url is surfaced as a clickable link; the extension embeds no viewer
  devidp: the login url of requirement:contrib-devidp is surfaced the same way
non_goals:
  - a UI over api:cli-init or api:cli-new, which are interactive and belong in a terminal
  - a settings editor for popcornwave.toml
  - replacing the api:cli-dev terminal with a custom panel
acceptance:
  - a generation error selected in the Problems view opens the source file at the failing position, not the generated file
  - api:cli-dev started twice yields one loop
  - every command is disabled in an untrusted workspace
  - a command that would write announces what it writes before running, and api:cli-migrate additionally asks
  - the pw binary the commands run is the one policy:editor-tool-execution resolved, reported once at activation
```
