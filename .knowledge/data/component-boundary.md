---
id: data:component-boundary
type: data
title: Component Boundary Metadata
---
Each refreshable server-component boundary carries server-verifiable identity, version, dependency, and hierarchy metadata.

```yaml
fields:
  - component type ID
  - instance key
  - normalized input hash
  - output version or hash
  - dependency keys or tags
  - parent and child boundary relationship
trust:
  client_authority: none
  server_binding:
    - current route
    - session and authorization scope
    - build version
```
