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
    toolchain: tinygo or go, defaulting to tinygo
  dev:
    extra_watch: []
    idp:
      enabled: false
      config: devidp.toml
      port: 0 for an automatically reserved loopback port
    otel:
      enabled: true
      port: 0 for an automatically reserved loopback port
      max: 0 for the system:localotelviewer retention default
  migration:
    dir: migrations
    auto: true for api:cli-dev only
  assets:
    tailwind:
      enabled: false
      input: assets/app.css
      output: public/generated/app.css
      minify: true for api:cli-build
optional_extensions:
  - generated output rules
  - generated test policy
  - build tags and targets
  - build output location
rules:
  - api:cli-generate always discovers all Go, .pw.html, and .pw.sql sources
  - api:cli-dev always watches Go, .pw.html, .pw.sql, popcornwave.toml, and policy:config-file-resolution project-local files
  - project.toolchain records the compiler api:cli-init scaffolded for and rejects any other value
  - a missing project.toolchain means tinygo, because every project scaffolded before the key used api:serve-mux
  - dev.extra_watch adds relative files or glob patterns
  - migration.dir locates data:migration-source and is a tooling path, not a runtime database value
  - migration.auto only enables the api:cli-dev apply step and never enables application startup apply
  - dev.idp only affects api:cli-dev and locates data:devidp-config
  - dev.idp.port defaults to an automatically reserved port because api:cli-dev injects the resolved issuer into the application
  - dev.idp.enabled true requires the data:devidp-config file to exist
  - dev.otel only affects api:cli-dev and configures requirement:dev-telemetry-viewer
  - dev.otel.port defaults to an automatically reserved port because api:cli-dev injects the resolved endpoint, as it does for dev.idp
  - dev.otel.max bounds retained records per signal and zero keeps the viewer default
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
  file_selection: policy:config-file-resolution
  inputs:
    - TOML selected by configbind
    - environment
    - CLI flags
```
