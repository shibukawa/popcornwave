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
  - concept:page-tree roots, their reserved files, and their optional page.go
  - dynamo-tagged struct declarations and their dynamobind call sites
  - .pw.dynamo query declarations
flow: flow:generation-pipeline
discovery_scope:
  per_purpose: the data:project-config generate.handlers, generate.templates, generate.queries, generate.config, generate.pages, and generate.dynamo lists, per decision:explicit-generation-sources
  effect: a directory contributes only the artifact kinds whose purpose lists it, so a query directory is never analyzed for routes
  pages_unit: a generate.pages entry is walked as one concept:page-tree per flow:page-route-generation, not as a directory of independent sources
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
  from_generate_pages:
    - compiled page and layout components
    - the route decoder of each page
    - the api:page-registry and data:page-route-table of each tree root
    - api:page-action-endpoint registrations
    - request binders for the route packages, so an action can call pw.Parse
    - no OpenAPI, per decision:dual-router-coexistence
  from_generate_dynamo:
    - item codecs, key builders, and table definitions, per requirement:dynamodb-generation
    - the decision:dynamodb-table-registry list in the project.main package
    - no SQL dialect input, because there is no engine variant to compile for
  from_every_purpose: data:route-table, the exported view of the same route analysis
  optional: generated tests
check_mode:
  writes: none
  failure: generated content differs or is missing
behavior:
  - read a source only where the purpose that owns its kind lists its directory
  - walk each generate.pages root once, reporting every discovery problem in that walk rather than only the first
  - use the pw emitter of decision:page-render-binding for every page tree artifact, so generated pages call api:page-render-runtime rather than system:tinybind
  - run request binding over the packages a discovered tree reports, skipping the ones the generator reports nothing to generate for
  - register the Popcorn Wave generated header prefix with every discovery pass, so nothing this command wrote is analyzed as a source on the next run
  - keep, per directory, only the artifacts whose purpose lists that directory
  - warn once per .pw.html, .pw.sql, or stale generated file found outside its purpose, naming the path and the key
  - use system:tinybind route and call analysis behind the pw API
  - process sources and packages in stable lexical order
  - stop on parse or generation error
  - format generated Go source
  - replace destination files atomically after all generation succeeds
  - emit {source-base}_pw_gen.go beside each source
```
