---
id: api:cli-generate
type: api
title: petitweb generate
---
petitweb generate invokes the httpbind-go generator for every configured application package.

```yaml
usage: "petitweb generate [--check] [package ...]"
default_packages: data:project-config packages
flow: flow:generation-pipeline
artifacts:
  - httpbinder_gen.go
  - httpbinder_openapi_gen.go when enabled
  - petitweb_template_gen.go for each configured template package
check_mode:
  writes: temporary files only
  failure: committed generated content differs or is missing
behavior:
  - supply the pinned contrib/httpmux analysis adapter to system:httpbinder
  - process packages in stable lexical order
  - stop on parse or generation error
  - format generated Go source
  - replace destination files atomically after all generation succeeds
```
