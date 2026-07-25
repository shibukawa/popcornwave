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
  - run api:cli-migrate up when data:project-config migration.auto is enabled
  - start flow:tailwind-css-build watch mode when enabled
  - enable decision:development-public-assets
  - build and run data:project-config project.main
  - watch every Go, .pw.html, .pw.sql, popcornwave.toml, and config.toml source
  - watch the data:migration-source directory
  - add data:project-config dev.extra_watch paths and globs
  - exclude public/** and public/**/*.zstd from Go rebuild inputs
  - regenerate when generated inputs change
  - reapply pending migrations before restart when migration sources changed
  - rebuild and restart after successful changes
services:
  default: Valkey
  rule: default services may be disabled or changed in Devbox configuration
migration:
  default: enabled and forward-only under policy:migration-safety
  ordering: migrations complete before the application process starts
failure:
  generation_css_or_build: keep the developer loop alive and report diagnostics
  migration: report diagnostics, skip the restart, and keep the developer loop alive
```
