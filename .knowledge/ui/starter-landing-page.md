---
id: ui:starter-landing-page
type: ui
title: Starter Landing Page
---
The page api:cli-init scaffolds is a designed landing page that tells an operator what the project already contains and what to do next, rather than a heading that proves the server answered.

```yaml
audience: actor:application-developer
ui:
  root:
    kind: browser
    id: screen.starter-landing
    title: the project name
    children:
      - kind: header
        id: identity
        label: project name, with Popcorn Wave and the framework version under it
      - kind: text
        id: source-pointer
        label: the file and template this page is rendered from, so the first edit is obvious
      - kind: list
        id: included
        label: what this project was scaffolded with
        columns:
          - capability
          - the answer the wizard recorded
        state: one row per decision:interactive-project-bootstrap answer that wrote something
      - kind: list
        id: next-steps
        label: what to do next
        children:
          - kind: text
            label: edit this page
          - kind: text
            label: add a handler or a page, naming api:cli-new
          - kind: text
            label: write a query or a migration, present only for a project with a store
          - kind: text
            label: install a declined capability, naming api:cli-add and the capabilities this project skipped
      - kind: list
        id: documentation
        label: links to the documentation site
        children:
          - kind: link
            id: docs-home
            target: https://shibukawa.github.io/popcornwave/
          - kind: link
            id: docs-guides
            label: the guide for the capability set this project selected
          - kind: link
            id: repository
            target: https://github.com/shibukawa/popcornwave
      - kind: region
        id: authentication-controls
        state: present only for a mode that serves a login, holding the sign-in, sign-out, and passkey controls api:cli-init already scaffolds
styling:
  tailwind_selected: a composed layout built from requirement:tailwind-css-integration utilities, since a project that pinned the toolchain should see it doing something on the first run
  tailwind_declined: the same structure over the application-owned CSS the scaffold writes, so declining Tailwind costs styling rather than the page
  rule: one template, whose classes are the only difference between the two answers, because two page scaffolds would drift
  scope: no framework-hosted stylesheet and no CDN; the page is application-owned from the moment it is written
routers:
  registered: the handlers tree page, rendered through the api:cli-init starter handler
  discovered: the concept:page-tree root page
  both: the discovered root, with the registered tree keeping its own example
  shared: the requirement:nested-html-templates document shell, unchanged
rules:
  - every fact on the page comes from the answers this project was scaffolded with, so a declined capability is never advertised as present
  - the page is a starting point to delete, and nothing in the framework reads it
  - links are literal and static, since a landing page that needs a lookup to render is not a starting point
  - no inline script, per the api:cli-init passkey scaffold rule that already binds controls by element id
  - written under rule:template-source-layout, which matters more here than anywhere else because this template is the first one an operator reads
non_goals:
  - a dashboard reading runtime state
  - a page that stops rendering once its starter content is removed
  - localized copy; the scaffold writes one language and the application owns it afterwards
```
