---
id: concept:client-component
type: concept
title: Client Component
---
An explicitly declared optional island owns browser-only state and events without requiring full-document hydration.

```yaml
requirements:
  - serializable typed props
  - local state and DOM events
  - modules loaded only for client boundaries used by the page, delivered through requirement:framework-script-assets
  - calls to api:server-action and flow:partial-refresh
  - separation from server-only functions and credentials
  - independent failure boundary where practical
baseline: presentational and server components emit usable HTML without JavaScript
```
