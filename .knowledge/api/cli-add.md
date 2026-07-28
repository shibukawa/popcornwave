---
id: api:cli-add
type: api
title: pw add
---
pw add installs a framework capability into an existing project, so a choice declined at api:cli-init is not a decision the project is stuck with.

```yaml
usage: "pw add [capability]"
requirement: requirement:incremental-project-capabilities
mode: decision:post-init-scaffold-wizard
inputs:
  capability: preselects the first wizard step; omitting it lists the capabilities the project still lacks
  answers: the capability-specific questions, asked in the wizard only
questions:
  capability: single-select over the capabilities the project does not already carry
  database_dsn:
    asked_when: the answers reach the database, directly or through a dependency
    default: sqlite://{project}.db per requirement:contrib-sqlite
  oidc_provider:
    asked_when: auth is selected
    choices: requirement:contrib-devidp local emulator, or an external provider left for the operator to fill in
    mode: oidc, the only authentication mode with an implementation
  review: lists every file to create, every configuration section to append, and every follow-up command
capabilities:
  devbox:
    writes:
      - devbox.json carrying the toolchain project.toolchain records, plus the Tailwind pin when it is enabled
      - devbox.lock
    consumer: api:cli-dev starts the services it declares, and skips the step entirely for a project without the file
  database:
    writes:
      - data:middleware-runtime-config rdb section in every environment configuration file present
      - the migration directory, holding the same starter schema api:cli-init writes when the project has none
      - the same starter .pw.sql api:cli-init writes, and the generate.queries entry that opens the purpose for it
    enables: data:migration-source, api:migration-runner, and .pw.sql generation
    leaves_alone: an existing migration set, which is the application's own schema
  redis-valkey:
    requires: devbox, because the answer writes nothing but a package in that environment
    writes:
      - Valkey package in devbox.json, which api:cli-dev exposes as the development server
      - the endpoint an application passes to requirement:contrib-auth-state-redis
    refuses: enabling session.backend redis, which data:session-runtime-config still defers
  auth:
    requires: database, because the login session store is the rdb backend
    writes:
      - rule:framework-owned-tables migration files for the session store and the authentication tables
      - data:authentication-runtime-config section in every environment configuration file
      - account resolver source that api:authentication-endpoints calls
      - data:devidp-config roster and data:project-config dev.idp when the local emulator is selected
    imports: the application already links plugin/auth through its account resolver, so no separate wiring step exists
  tailwind:
    writes:
      - assets.tailwind section in data:project-config for requirement:tailwind-css-integration
      - pinned decision:tailwind-host-toolchain package in devbox.json
      - assets/app.css entry point
    manual: the stylesheet link belongs in the application-owned document shell, so it is printed rather than injected
    without_devbox: the requirement is printed instead of pinned, naming the standalone CLI and its minimum version rather than the Devbox package identifier, because there is no package list to write to
detection: the requirement:incremental-project-capabilities probes; no capability list is recorded anywhere
versioning:
  migration_version: the next free version in the project migration directory
  rationale: an application that already applied 00001 through 00007 must not have those renumbered, so no version range is reserved for the framework
  identity: the name stem published by the owning package, which makes the file recognizable at any version
configuration_edits:
  form: append the missing section to each existing environment configuration file
  reason: operator comments and hand-tuned values must survive the edit
  conflict: an existing section of the same name stops the command
rules:
  - require a data:project-config project root
  - refuse a capability the project already carries, naming the file that proves it
  - detect an installed capability by migration name stem or configuration section rather than by version
  - offer a missing required capability together with the one selected, and refuse the pair when it is declined
  - never rewrite or renumber an existing migration
  - never overwrite an application-owned file; report the conflict and stop
  - write nothing when any step would fail, so a partial capability cannot reach a project
  - run api:cli-generate after a capability that adds generated sources
  - print the commands that finish the installation, starting with api:migration-runner
  - reject a capability whose mode has no implementation, matching api:cli-init
relations:
  init: api:cli-init writes the same files for a new project, from the same capability catalog
  flow: flow:capability-addition
  layout: concept:project-layout
  sibling: api:cli-new adds sources rather than capabilities
exit:
  success: 0
  canceled_wizard: 0 with a canceled notice and no files written
  no_terminal: nonzero with usage
  already_present_or_conflict: nonzero with the path and the reason
```
