---
id: decision:layered-web-runtime
type: decision
title: Layered Web Runtime
---
Implement web capabilities as additive layers so applications stop at the highest layer they need.

```yaml
layers:
  - core: requirement:shared-web-runtime
  - classic: concept:classic-web-style
  - modern_server_ui: concept:modern-server-ui
  - optional_client_ui: concept:client-component
constraints:
  - classic builds exclude component graphs, patching, hydration, and browser runtime
  - modern layers remain compatible with standard handlers and middleware
```
