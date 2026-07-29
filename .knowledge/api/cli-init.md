---
id: api:cli-init
type: api
title: pw init
---
pw init creates a runnable Popcorn Wave project with a shared document shell, representative handler, typed page template, SQL query, error pages, Devbox environment, and generated-artifact conventions.

```yaml
usage: pw init [myapp] [--interactive] [--tailwind|--no-tailwind] [--tinygo|--no-tinygo] [--devbox|--no-devbox] [--database|--no-database] [--db=sqlite|postgres|mysql] [--redis|--no-redis] [--auth=none|oidc|oidc-passkey|passkey] [--devidp|--no-devidp]
mode: decision:interactive-project-bootstrap
catalog: the capability questions are the requirement:incremental-project-capabilities catalog api:cli-add installs into an existing project
inputs:
  directory: project directory; omitting it starts the wizard
  flags: shortcut answers that also seed the wizard
questions:
  project_name: directory and Go module name
  tinygo_support:
    default: yes
    yes: api:serve-mux routing and the TinyGo toolchain in Devbox
    no: net/http.ServeMux routing and the host Go toolchain only
    rationale: TinyGo produces much smaller binaries and has the more complete wasm target
  tailwind: optional_css below
  devbox:
    default: yes
    asked_last: it decides how this machine gets its tools rather than what the project contains
    yes: devbox.json and devbox.lock pinning the toolchain and the services
    no: the operator keeps their own setup, such as mise, Docker Compose, Nix, Homebrew, or Scoop; the Valkey question is skipped with it
    consequence: without it nothing pins the decision:tailwind-host-toolchain version, so api:cli-init and api:cli-build name the requirement, the standalone CLI at version 4 or later, rather than the Devbox package identifier that only nixpkgs understands
  database:
    default: yes
    yes: data:middleware-runtime-config rdb section, the migrations directory, and the .pw.sql and migration examples
    no: no rdb section and no SQL example, leaving a project that renders and serves only
    rationale: the SQL example, the initial migration, and rule:framework-owned-tables migrations all need a database, so declining it removes them together
  database_engine:
    asked_when: the database answer is yes
    default: sqlite
    choices: sqlite, postgres, and mysql per requirement:database-engine-selection
    writes: the rdb DSN, the dialect of the scaffolded migration and .pw.sql example, and the development server package
    shortcut: --db, which conflicts with --no-database
  redis_valkey:
    default: yes
    requires: the Devbox environment, which is the only thing this answer writes to
    yes: Valkey in the Devbox environment for requirement:contrib-redis-valkey consumers
    no: no Valkey package, keeping the development environment minimal
  authentication:
    default: none
    requires: database, because the login session store is the rdb backend
    none: no data:authentication-runtime-config section is written
    oidc: auth.mode oidc
    oidc_passkey: auth.mode oidc_passkey per decision:authentication-bootstrap-strategy
    passkey_only: auth.mode passkey_only recorded with auth.enabled false, because no implementation exists yet
  oidc_provider:
    asked_when: the selected mode uses OIDC
    local_emulator: requirement:contrib-devidp enabled through data:project-config dev.idp, with a data:devidp-config starter roster
    external: empty issuer, client id, and client secret that the operator or the environment must supply
    rationale: a skipped question never applies its answer, so a provider choice cannot leak into a project without OIDC
outputs:
  - data:project-config
  - concept:project-layout
  - config.dev.toml for requirement:environment-switching
  - Go module and cmd/myapp/main.go
  - project.toolchain in data:project-config recording the selected compiler
  - the four decision:explicit-generation-sources purpose lists in data:project-config, each naming the directories this scaffold actually created
  - flow:handler-registration mux for the selected toolchain
  - handler registration and pw.Parse example
  - templates/document.pw.html shared document shell
  - .pw.html page and 400, 401, 403, 404, 409, 413, and 500 templates
  - .pw.sql query example, only when the database is selected
  - data:project-config project.database naming the selected engine, which api:cli-generate reads as its SQL dialect
  - migrations/00001_init.sql application schema as migration version 1, in the dialect of the selected engine
  - a rule:rdb-dsn-resolution engine blank import in main, only for an engine pw does not link itself
  - public directory with non-served .keep sentinel and stable public.go embedding scaffold
  - tinygohelper.go netdev registration for rule:tinygo-runtime-compatibility, only when TinyGo is selected
  - .gitignore excluding **/*_pw_gen.go generated application build inputs
  - .vscode/settings.json hiding **/*_pw_gen.go
  - Devbox configuration with Valkey when selected, TinyGo when selected, and the selected requirement:database-engine-selection server package, only when the Devbox environment is selected
  - data:authentication-runtime-config section for the selected authentication mode
  - data:devidp-config roster and data:project-config dev.idp when the local emulator is selected
  - api:authentication-endpoints blank import in main and a sign-in and sign-out control on the starter page
  - rule:framework-owned-tables migrations from the packages that own those tables, at the versions after the application schema, when the mode serves a login
  - data:middleware-runtime-config rdb settings carrying the requirement:database-engine-selection DSN for the chosen engine, because the scaffolded migrations and queries need a database, only when the database is selected
optional_css:
  tailwind:
    - configure requirement:tailwind-css-integration in data:project-config
    - add pinned decision:tailwind-host-toolchain package to Devbox
    - create assets/app.css and application-owned CSS output wiring
behavior:
  - start the wizard when no project name is given or --interactive is set
  - refuse the wizard and print usage when the session has no terminal
  - validate the project name and destination, in the wizard before any file is written
  - refuse to overwrite nonempty destinations by default
  - create files atomically
  - run api:cli-generate
  - scaffold classic rendering according to requirement:nested-html-templates
  - scaffold runtime database configuration for decision:config-driven-database when the database example is enabled
  - refuse an authentication mode without the database, because api:cli-add applies the same dependency
  - refuse --db together with --no-database, before anything is written
  - write the starter migration and .pw.sql example in the dialect of the selected engine, since decision:server-sql-support-tier does not translate between them
  - leave every declined capability to api:cli-add, which reaches the same file state later
next_steps:
  - cd myapp
  - devbox shell, only for a project with the Devbox environment
  - pw dev
  - a notice naming every declined capability, because a scripted run never sees the wizard say it
  - for a server engine, the server to start and the role and database to create
exit:
  success: 0
  wizard_canceled: 0 with a canceled notice and no files written
  invalid_input_or_collision: nonzero with actionable path
```
