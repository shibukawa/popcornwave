---
id: data:project-config
type: data
title: Project Configuration
---
popcornwave.toml contains only pw build and development tooling configuration; runtime application settings belong to configbind inputs.

```yaml
file: popcornwave.toml
schema:
  project:
    name: myapp
    main: ./cmd/myapp
  generate:
    html:
      - "**/*.pw.html"
    sql:
      - "**/*.pw.sql"
  dev:
    watch:
      - "**/*.go"
      - "**/*.pw.html"
      - "**/*.pw.sql"
      - popcornwave.toml
  assets:
    tailwind:
      enabled: false
      input: assets/app.css
      output: internal/static/app.css
      minify: true for api:cli-build
optional_extensions:
  - generated output rules
  - generated test policy
  - build tags and targets
  - build output location
rules:
  - relative paths resolve from the config file directory
  - unknown keys are errors
  - command flags override config values
  - missing config is an error except for api:cli-init
  - server, database, session, security, middleware, and observability runtime values are forbidden
  - enabled Tailwind validates requirement:tailwind-css-integration and decision:tailwind-host-toolchain
  - Tailwind plugins and their options belong to the CSS entry through requirement:tailwind-plugin-integration
  - the CLI must already be available from the entered Devbox environment
runtime_configuration:
  owner: api:runtime-configuration
  inputs:
    - TOML selected by configbind
    - environment
    - CLI flags
```
