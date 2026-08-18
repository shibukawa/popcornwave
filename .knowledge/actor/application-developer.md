---
id: actor:application-developer
type: actor
title: Application Developer
---
The application developer creates and operates a typed HTTP service with the Popcorn Web CLI and standard Go source.

```yaml
responsibilities:
  - define request and response structs
  - register literal net/http routes
  - implement business validation and handlers
  - run api:cli-generate before committing generated artifacts
  - verify generated drift with api:cli-check and run go test ./...
```
