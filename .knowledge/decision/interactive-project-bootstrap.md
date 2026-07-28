---
id: decision:interactive-project-bootstrap
type: decision
title: Interactive Project Bootstrap
---
api:cli-init asks its questions in a terminal wizard while every answer stays reachable as a one-shot shortcut flag, so new project options cost one wizard step instead of a new command.

```yaml
status: accepted
selection:
  wizard: no project name given, or --interactive
  shortcut: project name given, which keeps existing scripts non-interactive
  no_terminal: refuse the wizard and print usage instead of guessing answers
question_model:
  ordered_steps: text input and single-select steps
  conditional_steps:
    rule: a step may declare the answers it applies to, and is skipped when they do not hold
    effect: a skipped step is absent from navigation, the step counter, and the review screen, and its answer is never applied
    reason: a follow-up question must not leak an answer into a project that never asked it
  seeding: shortcut flags become the preselected wizard answers
  review: final screen lists every answer before anything is written
  keys: arrow or jk to move, digits to jump, enter to accept, esc to go back, ctrl+c to cancel
  extension_cost: one option field, one shortcut flag, one wizard step, and its scaffold branch
questions:
  - project name
  - TinyGo support, defaulting to yes for decision:stdlib-servemux parity
  - Tailwind CSS
  - database, defaulting to yes because the SQL and migration examples depend on it
  - Redis or Valkey in the development environment, defaulting to yes
  - authentication mode, defaulting to none and requiring the database
  - OIDC provider, asked only for an OIDC mode, choosing requirement:contrib-devidp or an external provider
implementation:
  library: github.com/charmbracelet/bubbletea with bubbles and lipgloss
  scope: host-only per decision:host-tools-target-runtime, so it never reaches application binaries
reused_by: decision:post-init-scaffold-wizard, which drives api:cli-add and api:cli-new from the same step machinery without shortcut-flag parity
rationale:
  - a wizard explains each trade-off where the operator decides it
  - shortcut parity keeps api:cli-init scriptable and CI friendly
  - a declarative step list keeps future options additive
non_goals:
  - remembering answers between runs
  - prompting for values that belong to data:project-config edits later
```
