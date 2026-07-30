---
id: concept:modern-server-ui
type: concept
title: Modern Server UI
---
The server renders addressable typed component boundaries; a small browser runtime applies streamed and refresh patches without recreating the server component tree.

```yaml
component_roles:
  presentational: supplied values to server HTML
  server: typed data fetch to server HTML
  client: optional browser state and events via concept:client-component
server_component_input:
  properties: small, stable, data-identifying
  uses:
    - cache key
    - refresh target
    - instance identity
    - dependency analysis
capabilities:
  - api:typed-external-function
  - api:async-html-value
  - requirement:async-html-rendering
  - flow:initial-streaming-render
  - flow:partial-refresh
  - api:server-action
```
