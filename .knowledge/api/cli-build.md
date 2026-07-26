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
  - run flow:public-asset-build
  - resolve project.main and optional build settings from data:project-config
  - reject the build when the dependency graph of project.main contains a development-only package such as requirement:contrib-devidp
  - run go build with the resolved settings
defaults:
  package: data:project-config project.main
failure:
  - preserve previous successful output
  - return compiler diagnostics and nonzero status
  - an unlistable dependency graph skips the development-only check and lets the compiler report the real diagnostic
```
