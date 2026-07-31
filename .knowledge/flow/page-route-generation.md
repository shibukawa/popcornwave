---
id: flow:page-route-generation
type: flow
title: Page Route Generation
---
Generation walks each page tree root once and writes the components, decoders, and registry that serve it, in a run whose unit is the tree rather than the package directory.

```yaml
flow:
  trigger: api:cli-generate reads the data:project-config generate.pages roots
  steps:
    - id: discover
      action: walk one root, deriving a route from every directory holding page.pw.html, its ancestor layouts, and its segment kinds
      rules: rule:page-directory-naming
      reporting: every problem in the walk is reported, not only the first
    - id: analyze
      actor: system:tinybind
      action: read each page and layout component signature, read the optional page.go Load, and classify the concept:page-tree rung
      checks:
        - a Load matching neither the typed nor the handler signature
        - a typed Load whose results do not match the page component parameters by count, order, and type
        - a layout that does not declare children as html
        - an input type a URL cannot carry
    - id: discover-actions
      action: collect the exported handler-shaped functions of each route package as api:page-action-endpoint entries and reject a prefix an existing route occupies
    - id: emit-go
      emitter: the pw-owned emitter of decision:page-render-binding
      outputs:
        - page_pw_gen.go and layout_pw_gen.go compiled components
        - route_pw_gen.go route parameters and decoder
        - routes_pw_gen.go api:page-registry and data:page-route-table
      import_direction: down the tree only
    - id: bind-actions
      action: run request binding over every package the discovered tree reports, so api:page-action-endpoint handlers can call pw.Parse
      source: the tree's own package list, which is the route root plus each route and layout directory in deterministic order
      artifacts:
        kept: request binding
        dropped: the OpenAPI fragment, because a page route and an action endpoint are not a published contract
      empty_package: a package with nothing to bind reports the generator's nothing-to-generate error, which this step skips rather than failing on, since most route packages have no request model
      why_not_earlier: binder discovery reads the Bind call sites of the package it analyzes and never consults a registration, so the only thing that was missing is this run
    - id: compare-or-commit
      action: join these files with the rest of the api:cli-generate plan, then compare in check mode or replace atomically
      per_directory: the tree's output is planned as part of the directory it lands in, so one directory keeps one staleness sweep
      merge: a compiled component and a request binder that derive the same base name, as page.pw.html and page.go do, become one generated file rather than deleting each other
  failure:
    default: nothing is written for any purpose, since the plan commits as one set
  ordering: roots and directories are processed in stable lexical order, so identical sources produce identical bytes
openapi_exclusion:
  first_guard: the pages purpose keeps no OpenAPI artifact, which is the same per-directory artifact selection decision:explicit-generation-sources already applies
  second_guard: the Popcorn Wave generated header prefix is registered with the discovery pass, so nothing api:cli-generate wrote is read back as a source
  why_both: the second guard is a registration a future change could drop, and the first holds without it
  observed_upstream: once discovery reached the route packages, every page route and action endpoint appeared in the document, so this is a demonstrated failure rather than a precaution
  wider_effect: one registered prefix also covers the generated files of every other purpose, which discovery has been reading all along; they carry no registrations, so nothing was wrong, but a generated binder is not an input either
relation:
  pipeline: flow:generation-pipeline, of which this is the page-tree purpose
  templates: the flat flow:template-generation run compiles the handler-tree templates and never reaches inside a page tree root
```
