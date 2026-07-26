---
id: api:cli-init
type: api
title: pw init
---
pw init creates a runnable Popcorn Wave project with a shared document shell, representative handler, typed page template, SQL query, error pages, Devbox environment, and generated-artifact conventions.

```yaml
usage: pw init [myapp] [--interactive] [--tailwind|--no-tailwind] [--tinygo|--no-tinygo]
mode: decision:interactive-project-bootstrap
inputs:
  directory: project directory; omitting it starts the wizard
  flags: shortcut answers that also seed the wizard
questions:
  project_name: directory and Go module name
  tinygo_support:
    default: yes
    yes: api:serve-mux routing and the TinyGo toolchain in Devbox
    no: net/http.ServeMux routing and the host Go toolchain only
    rationale: TinyGo produces much smaller binaries and has the more complete wasm target
  tailwind: optional_css below
outputs:
  - data:project-config
  - concept:project-layout
  - config.dev.toml for requirement:environment-switching
  - Go module and cmd/myapp/main.go
  - project.toolchain in data:project-config recording the selected compiler
  - flow:handler-registration mux for the selected toolchain
  - handler registration and pw.Parse example
  - templates/document.pw.html shared document shell
  - .pw.html page and 400, 401, 403, 404, 409, 413, and 500 templates
  - .pw.sql query example
  - migrations/00001_init.sql application schema as migration version 1
  - public directory with non-served .keep sentinel and stable public.go embedding scaffold
  - tinygohelper.go netdev registration for rule:tinygo-runtime-compatibility, only when TinyGo is selected
  - .gitignore excluding **/*_pw_gen.go generated application build inputs
  - .vscode/settings.json hiding **/*_pw_gen.go
  - Devbox configuration with Valkey enabled by default and TinyGo when selected
optional_css:
  tailwind:
    - configure requirement:tailwind-css-integration in data:project-config
    - add pinned decision:tailwind-host-toolchain package to Devbox
    - create assets/app.css and application-owned CSS output wiring
behavior:
  - start the wizard when no project name is given or --interactive is set
  - refuse the wizard and print usage when the session has no terminal
  - validate the project name and destination, in the wizard before any file is written
  - refuse to overwrite nonempty destinations by default
  - create files atomically
  - run api:cli-generate
  - scaffold classic rendering according to requirement:nested-html-templates
  - scaffold runtime database configuration for decision:config-driven-database when the database example is enabled
next_steps:
  - cd myapp
  - devbox shell
  - pw dev
exit:
  success: 0
  wizard_canceled: 0 with a canceled notice and no files written
  invalid_input_or_collision: nonzero with actionable path
```
