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
    - registered
    - discovered
  router_pair: registered and discovered are the two routers the one api:cli-init router question selects between, per decision:page-router-scaffold-choice, so either can be installed into a project that started with only the other; they are named after the router rather than after the directory it reads
  dependencies:
    auth: database, because its session store is the rdb backend of data:session-runtime-config
    redis-valkey: devbox, because the answer writes nothing but a package in that environment
  parameterized:
    database: carries the requirement:database-engine-selection engine, so the capability is one entry with an answer rather than three entries
detection:
  source: the project files that carry the capability
  reason: a separate manifest in data:project-config would disagree with a hand-edited project
  probes:
    devbox: the devbox.json file
    database: middleware.rdb in the data:middleware-runtime-config section of the environment configuration
    redis-valkey: the Valkey package in devbox.json
    auth: the rule:framework-owned-tables migration name stem, at any version
    tailwind: assets.tailwind.enabled in data:project-config
    discovered: the generate.pages entries, whatever directory they name
    registered: the generate.handlers entries, whatever directory they name
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
