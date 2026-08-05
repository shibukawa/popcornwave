---
id: requirement:init-presets
type: requirement
title: Project Bootstrap Presets
---
api:cli-init offers a named preset that answers every capability question at once, so the common project shapes cost one choice instead of ten, and Manual stays for the project no preset describes.

```yaml
audience: actor:application-developer
motivation:
  - the question list grew with every capability, and a first project now answers ten before it sees a file
  - the answers are not independent: a login implies a store, a store implies an engine, a Redis session implies a development server, so most combinations are wrong and a handful are the ones people want
  - a preset is a name for one of those handful, and naming it is also how it gets documented and recommended
  - the questions themselves stay; requirement:incremental-project-capabilities still reverses any answer afterwards
selection: decision:preset-first-bootstrap
manual_mode: decision:navigable-answer-hub
surface: ui:init-preset-hub
shortcut: --preset, per the decision:interactive-project-bootstrap parity rule
catalog:
  order: the presets first in the order below, Manual last
  website-login:
    label: Web site with login
    for: a website whose pages belong to whoever signed in
    router: discovered, per requirement:discovered-page-routing
    auth: oidc
    oidc_provider: the requirement:contrib-devidp local emulator, so login works before a real provider exists
    session: redis, taking the Valkey development server with it
    database: yes at sqlite, because plugin/auth keeps its ceremony records and its allowlist in SQL whatever holds the sessions
    tailwind: yes
    dynamo: no
    engine_is_a_default: sqlite is the answer that needs no server to start, and requirement:preset-customization-docs owns switching it
  website-aws:
    label: Web site on AWS
    for: the same website with no relational database to operate
    router: discovered
    auth: oidc with auth.backend dynamo, per decision:auth-backend-selection
    oidc_provider: the local emulator
    session: dynamo, per requirement:dynamodb-session-store
    database: no
    dynamo: yes
    tailwind: yes
    redis: no
    engine: none, because requirement:dynamodb-auth-backend moved all four stores plugin/auth owns onto DynamoDB, so the login no longer drags a SQL engine behind it
    was_gated:
      until: 2026-08-05, when requirement:dynamodb-auth-backend shipped
      what_was_missing: the authstate/dynamo adapter, the allowlist seam, the plugin/auth gate on middleware.rdb, and a dynamo answer in the session step
      all_answered: requirement:contrib-auth-state-dynamo and requirement:dynamodb-auth-stores are built, the gate is now asserted per backend rather than by the package, and api:cli-init offers dynamo as a session backend
      not_substituted: a version carrying a SQL engine beside DynamoDB was rejected while it was blocked, and is now unnecessary rather than merely unwanted
  website-discovered:
    label: Simple website
    for: a website with no accounts and nothing to store
    router: discovered
    auth: none
    session: cookie
    database: no
    dynamo: no
    tailwind: no
  website-registered:
    label: Simple website, handlers
    for: the same site written as Go registrations rather than a page tree
    router: registered
    differs_from_website-discovered: the router answer only
    pairing_reason: decision:page-router-scaffold-choice is the one bootstrap answer with no wrong choice, so the preset list names both rather than picking for the reader
    auth: none
    session: cookie
    database: no
    dynamo: no
    tailwind: no
  api-server:
    label: API Server
    for: a machine-facing API whose callers arrive with a token somebody else issued
    router: registered, because an API is registrations and the OpenAPI document they produce
    auth: jwt_only, per requirement:jwt-only-api-authentication
    scaffold: requirement:api-server-scaffold
    reverses: decision:jwt-only-mode-not-scaffolded, which held that no command offers this mode; decision:jwt-only-preset-scaffolding is the amendment and states what changed
    session: cookie, the answer that stores nothing, because the mode takes no session for authentication and a preset should not provision storage nothing reads
    database: no, which follows from the scaffold leaving revocation off
    tailwind: no
    dynamo: no
    redis: no
  package:
    label: Package
    for: a concept:component-package published as a Go module, with no application of its own
    kind: project.kind package, per api:cli-package, which is the same scaffold this preset names
    scaffold: requirement:package-project-scaffold
    answers_nothing_else: the capability questions describe an application, and this preset has none to answer
    committed_artifacts: decision:committed-package-artifacts, which is the one rule this project kind inverts
  manual:
    label: Manual
    for: any project the six above do not describe
    answers: nothing; it opens decision:navigable-answer-hub on the defaults
shared_by_every_preset:
  tinygo: yes
  devbox: yes
  project_name: the one question a preset still asks
  reason: neither answer changes what the project contains, so neither distinguishes one preset from another
  except_package: the package preset answers neither, because it produces no binary to compile and declares no services to run
not_all_presets_are_answer_sets:
  rule: a preset may set project.kind, and a preset that does removes questions rather than answering them
  instance: the package preset, whose project has no entry point, no router, no login, and no environment configuration to ask about
  effect_on_the_hub: decision:navigable-answer-hub shows the rows that apply to the kind, using the same rule that hides a conditional step
  why_not_a_separate_command: a reader deciding what to create is already looking at this list, and a second command they have to know the name of is the discovery problem the presets exist to solve
not_a_capability:
  rule: a preset is not a member of the requirement:incremental-project-capabilities catalog
  reason: api:cli-add installs one capability into a project that exists; a preset is a set of answers given once, and there is nothing to add later
  effect: pw add takes no preset argument, and a project records no preset it was created from
acceptance:
  - each preset produces the same project as answering the wizard with its listed answers
  - a preset asks the project name and nothing else before the review
  - the review names the preset and lists every answer it decided
  - a preset answer set that the validation rules of api:cli-init would refuse never reaches the list
  - --preset with any other shortcut flag reports the conflict before writing, since a preset is the answer to the questions those flags answer
non_goals:
  - saving the answers of a Manual run as a new preset, which vue-cli does and which needs a place to write it that api:cli-init does not have
  - a preset per engine, per session backend, or per authentication mode, which is the question list again under another name
  - recording the preset in data:project-config
```
