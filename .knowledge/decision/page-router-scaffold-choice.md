---
id: decision:page-router-scaffold-choice
type: decision
title: Page Router Scaffold Choice
---
api:cli-init asks which router or routers a project starts with, and both answers stay installable afterwards, because decision:dual-router-coexistence makes the pair a scaffold question rather than a framework mode.

```yaml
status: accepted
vocabulary:
  router_level:
    names: registered, discovered, and both
    used_by: the api:cli-init flag, its wizard step, and the api:cli-add catalog
    rule: a router is named after how it gets its routes, never after the directory it reads; the directory is a data:project-config value
  source_level:
    names: handler and page
    used_by: the api:cli-new kinds
    rule: a source is named after what it is, never after the router that will serve it
  why_two_sets: the commands work at different levels, so one word for both would name a router where a file is meant; pw add installs a router, and pw new adds one source to a router the project already has
  crossing: a message that has to cross the two levels says both words rather than blending them, as pw new does when it points at pw add
question:
  key: router
  asked: after the project name and toolchain, before the capability questions, because it decides which source trees the rest of the scaffold writes into
  default: registered, which is the shape every existing project already has
  shortcut: --router=registered|discovered|both
answers:
  registered:
    label: registered router, for HTML, API, traditional Go style, and OpenAPI
    writes: the handlers tree of flow:handler-registration and its generate.handlers and generate.templates entries
  discovered:
    label: discovered router, for an HTML website only
    writes: the concept:page-tree root and its generate.pages entry, with generate.handlers left empty
  both:
    label: both, an API in the handlers tree and a website in the page tree
    writes: both trees, which coexist without further configuration
directories:
  scaffolded: handlers for the registered router and pages for the discovered one
  status: defaults rather than fixed names; api:cli-generate, api:cli-new, and api:cli-dev all read the decision:explicit-generation-sources purpose lists
  renaming: move the directory and edit its purpose entry; nothing else refers to the name
  package_names: a generated package name follows its directory, so a renamed tree keeps compiling
invariants:
  - templates/document.pw.html and the error pages are written for every answer, because decision:implicit-document-shell and flow:error-template-generation serve both routers
  - the pages answer still scaffolds concept:application-entry-point unchanged; only what it starts differs
  - a declined tree leaves an explicit empty purpose list rather than a missing key, per decision:explicit-generation-sources
reversibility:
  requirement: requirement:incremental-project-capabilities
  capabilities: registered and discovered, the same three words the question uses, so either router can be installed into a project that declined it
  detection:
    discovered: the generate.pages entries, whatever directory they name
    registered: the generate.handlers entries, whatever directory they name
  refused: removing a router from a project that has one
rationale:
  - a website author who never writes an API should not have to read the handler scaffold to find the page one
  - an API project gains nothing from an empty page tree, and an unused tree root is a directory whose purpose nobody can state
  - one question with three answers costs one wizard step, per decision:interactive-project-bootstrap
non_goals:
  - a project-wide router mode that changes framework behavior
  - converting a handler into a page or the reverse
```
