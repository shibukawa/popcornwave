---
id: ui:init-preset-hub
type: ui
title: pw init Preset and Answer Hub
---
The three screens api:cli-init shows in a terminal: the requirement:init-presets list, the project name, and the decision:navigable-answer-hub that both reviews a preset and edits a Manual answer set.

```yaml
ui:
  root:
    kind: terminal
    id: screen.init
    title: Popcorn Wave  new project
    children:
      - kind: screen
        id: preset-list
        title: Preset
        state: first screen of every run with a terminal
        children:
          - kind: choice-list
            id: presets
            columns:
              - label
              - summary
            options:
              - label: Web site with login
                summary: discovered routing, OIDC, Redis sessions, SQLite, Tailwind
              - label: Web site on AWS
                summary: discovered routing, OIDC, DynamoDB, Tailwind
              - label: Simple website
                summary: discovered routing, no login, no database
              - label: Simple website, handlers
                summary: registered routing, no login, no database
              - label: API Server
                summary: registered routing, JWT verification, no browser login
              - label: Package
                summary: a publishable module, no application of its own
              - label: Manual
                summary: choose every answer
          - kind: footer
            label: "↑/↓ move  ·  enter next  ·  ctrl+c cancel"
      - kind: screen
        id: project-name
        title: Project name
        state: shown for every preset including Manual
        children:
          - kind: text-input
            id: name
            label: Creates ./<name> holding a Go module of the same name
            state: seeded by the directory argument
          - kind: text-input
            id: module-path
            label: The module path a consumer imports; the directory is its last element
            state: replaces the name input for the package preset, per requirement:package-project-scaffold
      - kind: screen
        id: hub
        title: Review
        state: reached from the name step; the same screen for a preset and for Manual
        children:
          - kind: table
            id: answers
            columns:
              - question
              - answer
              - answered
            action: enter opens the question of the selected row
            target: screen.init/step
          - kind: row
            id: create
            label: create
            action: flow:project-bootstrap
          - kind: footer
            label: "↑/↓ move  ·  enter open  ·  ctrl+c cancel"
      - kind: screen
        id: step
        title: one question
        state: the existing choice or text step, opened from a hub row
        children:
          - kind: footer
            label: "↑/↓ move  ·  enter accept  ·  esc back  ·  ctrl+c cancel"
        action: returns to screen.init/hub on accept and on esc
```

```yaml
presentation:
  preset_summary: one line naming the answers that distinguish the preset, not every answer it gives
  hub_answer_column: the same value the review screen renders today, per the wizardStep value contract
  hub_answered_column: blank for a default nobody opened, and a mark for an answer the operator gave or a preset decided
  unshippable_preset: a preset whose capability is not built is absent from the list rather than shown disabled; none is absent today
  no_terminal: none of these screens; the shortcut path prints nothing interactive
copy_rule: each question keeps the explanation it already carries, since the hub shows the label and the step shows the reasoning
```
