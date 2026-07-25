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
  - enable decision:development-public-assets
  - build and run data:project-config project.main
  - default data:runtime-environment to dev when APP_ENV is unset
  - watch every Go, .pw.html, .pw.sql, popcornwave.toml, config.*.toml, and config/config.*.toml source
  - add data:project-config dev.extra_watch paths and globs
  - exclude public/** and public/**/*.zstd from Go rebuild inputs
  - regenerate when generated inputs change
  - rebuild and restart after successful changes
services:
  default: Valkey
  rule: default services may be disabled or changed in Devbox configuration
failure:
  generation_css_or_build: keep the developer loop alive and report diagnostics
```
