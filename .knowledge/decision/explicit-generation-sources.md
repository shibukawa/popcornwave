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
  generate.pages:
    reads: a concept:page-tree root, walked as one tree rather than as a directory of sources
    emits: flow:page-route-generation components, decoders, the api:page-registry, and the request binders of the route packages
    unit: an entry is a tree root, so one entry is one generation run and one registry
    artifact_selection: request binding is kept and the OpenAPI fragment is dropped, which is the same per-directory selection every other purpose makes and is what keeps a page route out of the document
    disjoint_from_templates: a directory below a root is never a generate.templates entry, because the tree run already compiles its page and layout templates and the flat run would claim the same output with different content
    disjoint_from_handlers: a root is never a generate.handlers entry, so no page route is analyzed for OpenAPI, per decision:dual-router-coexistence
  generate.dynamo:
    reads: Go sources carrying dynamo struct tags, the dynamobind call sites that direct which half of each codec is emitted, and .pw.dynamo query declarations
    emits: requirement:dynamodb-generation item codecs, key builders, table definitions, the decision:dynamodb-table-registry list, and the requirement:dynamodb-typed-queries functions
    not_queries: generate.queries reads .pw.sql for a SQL dialect, and this purpose reads Go type declarations plus a declaration language checked against those types, so one scanner could not serve both
    overlaps_handlers: a directory may be both, because a tagged record commonly lives beside the handler that stores it
form:
  value: project-relative directories, listed explicitly and walked recursively
  required: the four original keys, so the block states every purpose and its scope in one place
  empty_list: the explicit way to say a purpose generates nothing, distinguishable from a forgotten key
  pages_exception: generate.pages and generate.dynamo are the optional keys, and an absent one means the empty list, because a project written before the purpose existed cannot have named it
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
  - a generate.pages root inside another generate.pages root, which would serve one page from two registries
  - a generate.pages root that is also a generate.templates or generate.handlers entry, per the disjointness above
fixed_paths:
  reason: these are addressed directly rather than discovered, so they are outside every purpose
  members:
    - the project.main directory, which receives the generated registration linker
    - the project-root public.go of requirement:public-asset-delivery
    - the project-root asset manifest flow:public-asset-build writes into the package that public.go declares, which cannot move into a purpose because no generation run produces it
    - the requirement:template-storybook harness directory, generated into a directory of its own
outside_sources:
  reported:
    - .pw.html outside every generate.templates entry
    - .pw.html inside a generate.pages root under a name concept:page-tree does not reserve, which nothing compiles
    - .pw.sql outside every generate.queries entry
    - .pw.dynamo outside every generate.dynamo entry
    - a policy:generated-artifacts file outside every purpose, which nothing regenerates or removes any more, except the fixed_paths above
  not_reported: Go sources, because ordinary Go code lives throughout a project; a call site outside its purpose simply has no generated binding
  behavior: warn and ignore
  fixed_paths_are_exempt:
    rule: a generated file at a fixed path is written deliberately by something other than a generation run, so it is never stale and is never reported
    found_by: reporting the asset manifest printed a warning on every rebuild after the first, in every scaffolded project, about a file the same build had just written
    bound: the exemption is the exact path, so a copy left anywhere else is still reported
  message: names the path and the purpose key that would include it
  rejected_alternative: failing the build, which would break a project that keeps deliberate samples or fixtures beside its code
consumers:
  generation: api:cli-generate keeps, per directory, only the artifacts whose purpose lists that directory
  developer_loop: api:cli-dev regenerates from these lists but watches wider, per decision:developer-loop-watch-scope
  scaffolding: api:cli-new offers destinations inside the purpose its artifact belongs to, which for a page is a generate.pages root
rationale:
  - a reader answers "what is this directory generated for" by reading the purpose it appears under
  - a purpose that scans an unrelated tree pays for it on every developer-loop rebuild
  - an explicit list makes an unscanned source a reportable condition rather than silence
  - the config purpose can reach the main package without the handler purpose reaching it too
migration:
  existing_projects: add the four original keys; the load fails until they are present, and generated files the old whole-tree scan left outside every purpose are reported as stale
  pages_key: a project written before requirement:discovered-page-routing has no generate.pages, which means the empty list, because a project with no page tree is the shape every existing project has
non_goals:
  - per-file include and exclude globs
  - a generated list that the CLI maintains on the operator's behalf
  - changing which file extensions are eligible inside a listed directory
  - a purpose per artifact variant, such as splitting binding from its OpenAPI fragment
```
