---
id: decision:explicit-generation-sources
type: decision
title: Explicit Generation Sources
---
data:project-config scopes api:cli-generate per generation purpose, so each kind of generated code names the directories it may come from instead of every purpose inheriting one walk of the whole project.

```yaml
status: accepted
replaces: whole-project discovery excluding only .git, vendor, node_modules, and .devbox
purposes:
  reason: the generator already emits one artifact kind per purpose, so a single list would give a query scanner the handler tree and a config scanner the template tree
  generate.handlers:
    reads: route registrations, pw.Parse call sites, and response calls in Go sources
    emits: request binding and the OpenAPI fragment of api:cli-generate
    contract: rule:static-route-discovery
  generate.templates:
    reads: .pw.html
    emits: flow:template-generation renderers
    framework_lookup: this list, and only this list, is where requirement:nested-html-templates resolves the document shell and flow:error-template-generation resolves the error pages
  generate.queries:
    reads: .pw.sql
    emits: flow:sql-generation query functions
  generate.config:
    reads: pw.RegisterConfig and api:subcommands RegisterSubCommand call sites in Go sources
    emits: configuration and subcommand binding
    typical_entry: the project.main directory, because concept:application-entry-point is where most projects register both
form:
  value: project-relative directories, listed explicitly and walked recursively
  required: all four keys, so the block states every purpose and its scope in one place
  empty_list: the explicit way to say a purpose generates nothing, distinguishable from a forgotten key
  overlap: a directory may appear under several purposes, because concept:project-layout keeps a page template beside the handler that renders it
  scaffolded: written by api:cli-init for the directories it creates
  no_implicit_default: nothing is scanned because it happens to be inside the project
  missing_key: an error naming an example, because a fallback would be the implicit scope this replaces
rejected_entries:
  - an absolute path
  - the project root or a path that leaves it
  - a directory listed twice under one purpose
  - a directory already covered by another entry of the same purpose, whose sources would be planned and then deleted twice
  - a path that is not an existing directory, so a typo is reported instead of silently generating nothing
fixed_paths:
  reason: these are addressed directly rather than discovered, so they are outside every purpose
  members:
    - the project.main directory, which receives the generated registration linker
    - the project-root public.go of requirement:public-asset-delivery
outside_sources:
  reported:
    - .pw.html outside every generate.templates entry
    - .pw.sql outside every generate.queries entry
    - a policy:generated-artifacts file outside every purpose, which nothing regenerates or removes any more
  not_reported: Go sources, because ordinary Go code lives throughout a project; a call site outside its purpose simply has no generated binding
  behavior: warn and ignore
  message: names the path and the purpose key that would include it
  rejected_alternative: failing the build, which would break a project that keeps deliberate samples or fixtures beside its code
consumers:
  generation: api:cli-generate keeps, per directory, only the artifacts whose purpose lists that directory
  developer_loop: api:cli-dev regenerates from these lists but watches wider, per decision:developer-loop-watch-scope
  scaffolding: api:cli-new offers destinations inside the purpose its artifact belongs to
rationale:
  - a reader answers "what is this directory generated for" by reading the purpose it appears under
  - a purpose that scans an unrelated tree pays for it on every developer-loop rebuild
  - an explicit list makes an unscanned source a reportable condition rather than silence
  - the config purpose can reach the main package without the handler purpose reaching it too
migration:
  existing_projects: add the four keys; the load fails until they are present, and generated files the old whole-tree scan left outside every purpose are reported as stale
non_goals:
  - per-file include and exclude globs
  - a generated list that the CLI maintains on the operator's behalf
  - changing which file extensions are eligible inside a listed directory
  - a purpose per artifact variant, such as splitting binding from its OpenAPI fragment
```
