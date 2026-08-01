---
id: ui:tutorial-memo-app
type: ui
title: Tutorial Memo Application
---
The memo application the tutorial builds is styled as it is typed, because a reader who follows every step to the end should be looking at something they would show someone.

```yaml
audience: actor:application-developer
surface: the running example of requirement:tutorial-continuity, carried across the tutorial chapters
ui:
  root:
    kind: browser
    id: screen.tutorial-memos
    title: Memos
    children:
      - kind: form
        id: compose
        target: POST /memos
        children:
          - kind: textarea
            id: body
            label: the next memo, holding the rejected draft when one comes back
          - kind: text
            id: field-error
            state: present only for a rejected submission, beside the field it belongs to
          - kind: button
            id: submit
            label: Add
      - kind: list
        id: memos
        label: what has been written, newest first
      - kind: region
        id: account
        state: present from the login chapter onward, holding the display name and the sign-out control
styling:
  toolchain: requirement:tailwind-css-integration, so the classes in the templates are the ones the project actually compiles
  defect_this_fixes:
    form: the forms chapter renders an unstyled textarea and button, which is the first page the reader builds themselves and the one that looks least finished
    dangling_class: its error paragraph carries class="error", which no stylesheet in the project defines
    scaffold: the api:cli-init starter page already ships text-3xl font-bold, Tailwind classes in a project the same tutorial set up without Tailwind
  rule: no template in the tutorial carries a class the project cannot compile, in either direction
project_toolchain:
  requirement: the tutorial project has Tailwind from the chapter that first shows a styled template
  mechanism: api:cli-add at the head of that chapter, per requirement:tutorial-capability-growth
  consequence: getting-started declines Tailwind, which is what makes the forms chapter have something to add
rules:
  - styling is shown, not taught; the chapters explain the framework and let the classes speak for themselves
  - a class appears in a template only where the chapter has already added the page it belongs to
non_goals:
  - a design system, a component library, or a theme
  - teaching Tailwind, which has its own documentation
  - restyling the api:cli-init scaffold from here; ui:starter-landing-page owns that page
```
