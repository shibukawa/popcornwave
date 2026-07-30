---
id: api:cli-generate
type: api
title: pw generate
---
pw generate scans Go, .pw.html, and .pw.sql sources and emits all required application mapping and codec code beside its source.

```yaml
usage: pw generate [--check]
inputs:
  - pw.Parse[T] call sites
  - route registrations
  - .pw.html files
  - .pw.sql files
  - reachable JSON types
flow: flow:generation-pipeline
discovery_scope:
  per_purpose: the data:project-config generate.handlers, generate.templates, generate.queries, and generate.config lists, per decision:explicit-generation-sources
  effect: a directory contributes only the artifact kinds whose purpose lists it, so a query directory is never analyzed for routes
  fixed: the project.main directory and the project-root public.go
  required: the keys have no default, so a project without them fails to load
sql_dialect:
  source: data:project-config project.database
  effect: .pw.sql sources compile to the placeholder syntax of that engine, per flow:sql-generation
  no_default: the value is passed through rather than assumed, because a wrong dialect fails at the first query rather than at generation
  outside: warn and ignore a .pw.html, .pw.sql, or stale generated file found outside its purpose; Go sources are not reported
  consumers: api:cli-new derives its default destination from this scope, and api:cli-dev regenerates from it
artifacts:
  from_generate_handlers:
    - request binding
    - optimized JSON codecs
    - OpenAPI fragments
  from_generate_templates: typed HTML renderers
  from_generate_queries: context-based SQL functions
  from_generate_config: configuration and subcommand binding
  from_every_purpose: data:route-table, the exported view of the same route analysis
  optional: generated tests
check_mode:
  writes: none
  failure: generated content differs or is missing
behavior:
  - read a source only where the purpose that owns its kind lists its directory
  - keep, per directory, only the artifacts whose purpose lists that directory
  - warn once per .pw.html, .pw.sql, or stale generated file found outside its purpose, naming the path and the key
  - use system:tinybind route and call analysis behind the pw API
  - process sources and packages in stable lexical order
  - stop on parse or generation error
  - format generated Go source
  - replace destination files atomically after all generation succeeds
  - emit {source-base}_pw_gen.go beside each source
```
