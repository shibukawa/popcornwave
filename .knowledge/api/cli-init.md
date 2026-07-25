---
id: api:cli-init
type: api
title: pw init
---
pw init creates a runnable Popcorn Wave project with a shared document shell, representative handler, typed page template, SQL query, error pages, Devbox environment, and generated-artifact conventions.

```yaml
usage: pw init myapp [--tailwind]
inputs:
  directory: required project directory
outputs:
  - data:project-config
  - concept:project-layout
  - Go module and cmd/myapp/main.go
  - handler registration and pw.Parse example
  - templates/document.pw.html shared document shell
  - .pw.html page and 400, 404, and 500 templates
  - .pw.sql query example
  - public directory with non-served .keep sentinel and stable public.go embedding scaffold
  - Devbox configuration with Valkey enabled by default
optional_css:
  tailwind:
    - configure requirement:tailwind-css-integration in data:project-config
    - add pinned decision:tailwind-host-toolchain package to Devbox
    - create assets/app.css and application-owned CSS output wiring
behavior:
  - validate the project name and destination
  - refuse to overwrite nonempty destinations by default
  - create files atomically
  - run api:cli-generate
  - scaffold classic rendering according to requirement:nested-html-templates
next_steps:
  - cd myapp
  - devbox shell
  - pw dev
exit:
  success: 0
  invalid_input_or_collision: nonzero with actionable path
```
