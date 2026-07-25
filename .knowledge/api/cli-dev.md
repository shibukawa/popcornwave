---
id: api:cli-dev
type: api
title: pw dev
---
pw dev runs the local service set and continuously regenerates, rebuilds, and restarts the configured application.

```yaml
usage: pw dev
steps:
  - start configured Devbox services
  - run api:cli-generate
  - start flow:tailwind-css-build watch mode when enabled
  - build and run data:project-config project.main
  - watch configured Go, HTML, SQL, generated CSS output, and tooling configuration
  - regenerate when generated inputs change
  - rebuild and restart after successful changes
services:
  default: Valkey
  rule: default services may be disabled or changed in Devbox configuration
failure:
  generation_css_or_build: keep the developer loop alive and report diagnostics
```
