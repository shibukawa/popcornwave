---
id: decision:interactive-project-bootstrap
type: decision
title: Interactive Project Bootstrap
---
api:cli-init asks its questions in a terminal wizard while every answer stays reachable as a one-shot shortcut flag, so new project options cost one wizard step instead of a new command.

```yaml
status: accepted
selection:
  wizard: every run that has a terminal, whether or not a project name was given
  name_argument: seeds the project name step rather than skipping the wizard, because the name is one answer out of ten and a caller who knows it still has not answered the store, authentication, or toolchain questions
  shortcut: --yes takes the flags and the defaults for every unanswered question, which is the only way to skip the wizard in a terminal
  no_terminal:
    with_name: the shortcut path, so an existing CI script keeps working unchanged
    without_name: refuse and print usage instead of guessing the name
  superseded: the earlier rule that a project name alone selected the shortcut path, which hid every question after the first from anyone who typed the name they already knew
question_model:
  ordered_steps: text input and single-select steps
  conditional_steps:
    rule: a step may declare the answers it applies to, and is skipped when they do not hold
    effect: a skipped step is absent from navigation, the step counter, and the review screen, and its answer is never applied
    reason: a follow-up question must not leak an answer into a project that never asked it
  seeding: shortcut flags and the project name argument become the preselected wizard answers
  review: final screen lists every answer before anything is written
  keys: arrow or jk to move, digits to jump, enter to accept, esc to go back, ctrl+c to cancel
  extension_cost: one option field, one shortcut flag, one wizard step, and its scaffold branch
questions:
  - project name
  - TinyGo support, defaulting to yes for decision:stdlib-servemux parity
  - router, defaulting to registered, per decision:page-router-scaffold-choice
  - Tailwind CSS
  - authentication mode, defaulting to none, asked before the stores because it is the answer that decides whether a store is optional at all
  - store, asked only with a login, offering the requirement:database-engine-selection engines and DynamoDB with no none among them, since a session has to live somewhere
  - database, asked only without a login, defaulting to yes because the SQL and migration examples depend on it
  - database engine, asked without a login when the database is taken, and asked with a login when DynamoDB was the store answer, because plugin/auth keeps its ceremony records and its allowlist in SQL whatever holds the sessions
  - DynamoDB, defaulting to no, asked wherever it was not already the store answer, because requirement:dynamodb-store is a second kind of store rather than a fourth engine
  - OIDC provider, asked only for an OIDC mode, choosing requirement:contrib-devidp or an external provider
  - Devbox environment, defaulting to yes
  - Redis or Valkey in the development environment, defaulting to yes and skipped without the Devbox environment it installs into
ordering:
  project_then_machine: the questions that shape the project come first, and the two about how this machine gets its tools close the wizard
  reason: declining Devbox changes nothing about the code, so it does not belong among the answers that do
  dependants_follow: a question whose answer only applies inside another one is asked right after it, which is why Valkey follows Devbox, the engine follows the database, and the provider follows the mode
  authentication_before_stores:
    rule: authentication is asked first, and the store questions follow from its answer
    superseded: asking the stores first, which made authentication a question a project could pass without ever seeing
    effect: every path asks about both kinds of store; only the wording changes, from whether to which
    pairing: the store question a login answers covers one kind, and the one after it covers the other, so neither is skipped by taking a branch
implementation:
  library: github.com/charmbracelet/bubbletea with bubbles and lipgloss
  scope: host-only per decision:host-tools-target-runtime, so it never reaches application binaries
reused_by: decision:post-init-scaffold-wizard, which drives api:cli-add and api:cli-new from the same step machinery without shortcut-flag parity
rationale:
  - a wizard explains each trade-off where the operator decides it
  - a question never asked is an option never discovered, and the questions after the name are the ones a first project most needs
  - shortcut parity keeps api:cli-init scriptable and CI friendly
  - a declarative step list keeps future options additive
non_goals:
  - remembering answers between runs
  - prompting for values that belong to data:project-config edits later
```
