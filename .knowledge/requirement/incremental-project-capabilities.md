---
id: requirement:incremental-project-capabilities
type: requirement
title: Incremental Project Capabilities
---
Every scaffold decision made at api:cli-init stays reversible afterwards, so a project grows a capability or a route through a reviewed command instead of hand-copied starter files.

```yaml
motivation:
  - a bootstrap answer is given before the project is understood, so declining a capability must not be permanent
  - copying auth, database, Valkey, or Tailwind wiring by hand drifts from the version the framework expects
  - a handler needs a mux registration, a literal route pattern, and an optional template that must agree with each other
scope:
  capabilities: api:cli-add over the same catalog api:cli-init offers
  artifacts: api:cli-new over the concept:project-layout sources a project adds repeatedly
  interaction: decision:post-init-scaffold-wizard
capability_catalog:
  shared: api:cli-init questions and api:cli-add capabilities are one list, so an option costs one entry rather than one per command
  members:
    - devbox
    - database
    - redis-valkey
    - auth
    - tailwind
    - dynamo
    - registered
    - discovered
  router_pair: registered and discovered are the two routers the one api:cli-init router question selects between, per decision:page-router-scaffold-choice, so either can be installed into a project that started with only the other; they are named after the router rather than after the directory it reads
  dependencies:
    auth: database, because its session store is the rdb backend of data:session-runtime-config
    redis-valkey: devbox, because the answer writes nothing but a package in that environment
    dynamo: nothing, per requirement:dynamodb-store; it is a second kind of store rather than an alternative to the first
  parameterized:
    database: carries the requirement:database-engine-selection engine, so the capability is one entry with an answer rather than three entries
  excluded:
    jwt_only: requirement:jwt-only-api-authentication is not a catalog member and auth never means it, per decision:jwt-only-mode-not-scaffolded; a mode whose whole installation is a configuration section is not a capability a command installs
detection:
  source: the project files that carry the capability
  reason: a separate manifest in data:project-config would disagree with a hand-edited project
  consumers: api:cli-add offers what the project lacks, and api:cli-doctor reports it
  probes:
    devbox: the devbox.json file
    database: middleware.rdb in the data:middleware-runtime-config section of the environment configuration
    redis-valkey: the Valkey package in devbox.json
    auth: the rule:framework-owned-tables migration name stem, at any version
    tailwind: assets.tailwind.enabled in data:project-config
    dynamo: middleware.dynamo in the environment configuration, and the generate.dynamo entries
    discovered: the generate.pages entries, whatever directory they name
    registered: the generate.handlers entries, whatever directory they name
entry_point_edits:
  rule: api:cli-add edits concept:application-entry-point rather than describing the edit, so a capability installed later reaches the file state one installed at bootstrap already has
  covers: the blank imports a store needs and the account seam call a login needs
  why_it_outranks_application_ownership: storage is opt-in by blank import, so configuration that names a backend does nothing until the binary links it; printing that step left api:cli-init and api:cli-add producing different projects from the same answers, and the difference surfaced as a startup error rather than as a missing file
  what_makes_it_safe:
    reviewed: the edit is planned, named on the decision:post-init-scaffold-wizard review screen with its path, and applied only after that screen is accepted
    spliced: the insertion point comes from the parser and the rest of the file is copied byte for byte, so comments, grouping, and hand formatting survive
    idempotent: an import or a call already present is not added again, so running a capability twice stacks nothing
    accumulating: two capabilities in one run build on each other's planned edit rather than each starting from what is on disk
  fallback: a file this cannot edit — an unparenthesized import block, no func main, a package that does not parse — goes back to printing the step, so the operator still learns what is missing
  still_printed: an edit that is not an import or a call in main, such as the page tree registration on a mux the command cannot name
behavior:
  - both commands run inside an existing project and fail without data:project-config
  - both ask their questions in a terminal wizard and write only after the review screen is accepted
  - both compute the full file set first and write nothing when any step would fail
  - neither overwrites an application-owned file; the conflict is reported and the command stops
  - a step the framework cannot perform safely becomes a printed follow-up command
acceptance:
  - a project initialized without a capability reaches the same file state through api:cli-add as one initialized with it
  - adding a capability the project already carries fails with the path that proves it is present
  - a canceled wizard leaves the project byte-identical
  - api:cli-new handler produces a route that api:cli-generate accepts under rule:static-route-discovery
  - api:cli-new page produces a route directory that api:cli-generate serves without any registration
  - a project initialized with one router tree reaches the same file state as one initialized with both after adding the other
  - a failed write leaves no partially installed capability and no orphan handler source
non_goals:
  - removing a capability from a project
  - changing the database engine of a project that already has one
  - upgrading a capability already installed at an older shape
  - remembering answers between runs
  - full flag parity with api:cli-init, per decision:post-init-scaffold-wizard
```
