---
id: concept:project-layout
type: concept
title: Generated Project Layout
---
The starter project keeps handlers, templates, SQL, and their generated Go files close together so ownership and code review remain obvious.

```yaml
layout:
  popcornwave.toml: data:project-config
  go.mod: Go module definition
  go.sum: Go dependency checksums
  .gitignore: excludes public/**/*.zstd and other build-only output
  devbox.json: development tools and default Valkey service
  devbox.lock: pinned Devbox dependencies
  assets/app.css: optional Tailwind CSS configuration and plugin declarations
  assets/plugins/*.mjs: optional pinned standalone-compatible Tailwind plugin modules
  public/: requirement:public-asset-delivery source tree
  public/.keep: non-served empty-tree embed sentinel
  public/generated/app.css: optional generated Tailwind output
  public/*.zstd: generated flow:public-asset-build sidecars
  public.go: api:cli-init scaffolded embedded PublicFS accessor
  cmd/myapp/main.go: concept:application-entry-point
  handlers/index.go: package mux and Handlers accessor
  handlers/home_handler.go: route registration, request types, and net/http handler
  handlers/home.pw.html: typed HTML source
  handlers/home_pw_gen.go: generated HTML and request mapping
  queries/users.pw.sql: named SQL source
  queries/users_pw_gen.go: generated context-based query functions
  templates/document.pw.html: requirement:nested-html-templates document shell
  templates/document_pw_gen.go: generated document Fragment and Wrapper
  templates/templates.go: handwritten package marker available before first generation
  templates/400.pw.html: client error page
  templates/404.pw.html: not-found page
  templates/500.pw.html: internal error page
  templates/400_pw_gen.go: generated error page renderer
  templates/404_pw_gen.go: generated error page renderer
  templates/500_pw_gen.go: generated error page renderer
ownership:
  scaffolded_once:
    - public.go
  handwritten:
    - popcornwave.toml
    - go.mod
    - .gitignore
    - devbox.json
    - optional assets/app.css
    - optional assets/plugins/*.mjs
    - public source files excluding *.zstd
    - cmd/myapp/main.go
    - handlers/*_handler.go
    - "**/*.pw.html"
    - "**/*.pw.sql"
  generated: policy:generated-artifacts
  asset_output: optional public/generated/app.css from flow:tailwind-css-build
rules:
  - generated Go is emitted beside its source
  - generated filenames use {source-base}_pw_gen.go
  - generated filenames never start with an underscore
  - templates/document.pw.html owns doctype, html, head, and body; its body contains an unnamed <slot />
  - classic page templates provide leaf content and do not duplicate the document shell
  - public/.keep preserves an otherwise empty public directory and is never externally reachable
  - generated public/**/*.zstd sidecars are ignored by version control
  - Popcorn Wave never rewrites scaffolded public.go after initialization
  - api:public-asset-middleware owns public asset delivery
  - default Tailwind scaffolding creates no package.json or Node package lockfile
```
