---
id: api:cli-build
type: api
title: pw build
---
pw build generates current source artifacts and builds the configured application entry point.

```yaml
usage: pw build
steps:
  - run api:cli-generate
  - run flow:tailwind-css-build production mode when enabled
  - resolve project.main and optional build settings from data:project-config
  - run go build with the resolved settings
defaults:
  package: data:project-config project.main
failure:
  - preserve previous successful output
  - return compiler diagnostics and nonzero status
```
