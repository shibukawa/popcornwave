---
id: data:project-config
type: data
title: Project Configuration
---
popcornwave.toml contains only pw build and development tooling configuration; runtime application settings belong to configbind inputs.

```yaml
file: popcornwave.toml
schema:
  project:
    name: myapp
    kind: application or package, defaulting to application; a package project is a concept:component-package and carries the data:component-package-manifest package section
    main: ./cmd/myapp, required for an application and absent for a package
    toolchain: tinygo or go, defaulting to tinygo
    database: sqlite, postgres, or mysql, defaulting to sqlite
  dev:
    watch:
      includes: []
      excludes: []
    idp:
      enabled: false
      config: devidp.toml
      port: 0 for an automatically reserved loopback port; api:cli-init writes a fixed one, because the issuer it appears in is part of the account identity
    otel:
      enabled: true
      port: 0 for an automatically reserved loopback port, which is the OTLP receiver rather than the viewer page
      max: 0 for the system:localotelviewer retention default
    logs:
      enabled: true
      directory: .log, relative to the file and used by requirement:local-jsonl-log-capture
    console:
      enabled: true
      port: 18081, a fixed loopback port scaffolded by api:cli-init as dev.idp.port is, placed beside the 18080 the identity provider already takes
      overlay:
        enabled: true
        reload: true
      launcher:
        enabled: true
        corner: bottom-left, and one of bottom-left, bottom-right, top-left, top-right
      storybook:
        enabled: true
      queries:
        enabled: true
      data:
        enabled: true
      assets:
        enabled: true
      api:
        enabled: true
  generate:
    handlers: [handlers] as scaffolded, per decision:explicit-generation-sources
    templates: [handlers, templates] as scaffolded, because a page template sits beside its handler
    queries: [queries] as scaffolded
    config: [cmd/myapp] as scaffolded
    pages: [pages] as scaffolded for a project with a concept:page-tree, and empty otherwise
    dynamo: [records] as scaffolded for a project with requirement:dynamodb-store, and empty otherwise
  migration:
    dir: migrations
    auto: true for api:cli-dev only
  seed:
    auto: true for api:cli-dev only
  packages:
    form: an array of tables naming the concept:component-package modules the application uses, per decision:declared-package-installation
    entry: module, the Go module path, which must also be in go.mod
    absent: the empty list, because a project that uses no package has nothing to name
  assets:
    tailwind:
      enabled: false
      input: assets/app.css
      output: public/generated/app.css
      minify: true for api:cli-build
optional_extensions:
  - generated output rules
  - generated test policy
  - build tags and targets
  - build output location
rules:
  - project.kind selects which commands apply and which keys are legal, per api:cli-package; a missing key means application, because it was the only kind before the key existed
  - a packages entry is what links its module, because api:cli-generate emits the blank import from it; a module in go.mod and not in this list is an ordinary dependency
  - a packages entry naming a module with no data:component-package-manifest package section is an error
  - api:cli-generate reads each source kind only under the generate purpose that owns it, and warns about a .pw.html, .pw.sql, or .pw.dynamo outside its purpose
  - every generate purpose key is required except generate.pages and generate.dynamo; an empty list states that the purpose generates nothing
  - a missing generate.dynamo means the empty list, for the same reason generate.pages does; requirement:dynamodb-store is a capability a project acquires rather than one it always had
  - project.database names the SQL engine only, and says nothing about requirement:dynamodb-store, which is configured at runtime and never here
  - a missing generate.pages means the empty list, because a project scaffolded before requirement:discovered-page-routing has no concept:page-tree and no way to acquire one silently
  - a generate.pages entry is a tree root, so it is neither nested in another root nor listed under generate.templates or generate.handlers
  - the scaffolded directory names are defaults, not identity: handlers and pages are what api:cli-init writes, and every consumer reads the purpose list instead of the name, so renaming a tree is moving the directory and editing its entry
  - a generated package name follows the directory it is in, so a renamed tree compiles without an edit to its sources
  - a generate entry is relative, names an existing directory, and is neither duplicated nor nested inside another entry of the same purpose
  - one generate.templates entry holds the requirement:nested-html-templates document shell, and a second one is an error
  - api:cli-dev regenerates from the generate purposes but watches per decision:developer-loop-watch-scope
  - dev.watch.includes adds relative files or glob patterns, and dev.watch.excludes skips directory subtrees
  - project.toolchain records the compiler api:cli-init scaffolded for and rejects any other value
  - a missing project.toolchain means tinygo, because every project scaffolded before the key used api:serve-mux
  - project.database records the requirement:database-engine-selection engine and rejects any other value
  - a missing project.database means sqlite, because it was the only engine that existed before the key
  - project.database is a generation input, not a runtime one; the effective engine still comes from the rule:rdb-dsn-resolution scheme, and the two must agree
  - migration.dir locates data:migration-source and is a tooling path, not a runtime database value
  - migration.auto only enables the api:cli-dev apply step and never enables application startup apply
  - seed.auto only enables the api:cli-dev reseed step and never seeds from application startup, api:cli-migrate, or a build
  - seed.auto has no directory key beside it; the datasets are the api:cli-seed default location, and its --dir flag stays the way a one-off run points elsewhere
  - dev.idp only affects api:cli-dev and locates data:devidp-config
  - dev.idp.port defaults to an automatically reserved port because api:cli-dev injects the resolved issuer into the application
  - dev.idp.enabled true requires the data:devidp-config file to exist
  - dev.otel only affects api:cli-dev and configures requirement:dev-telemetry-viewer
  - dev.otel.port defaults to an automatically reserved port because api:cli-dev injects the resolved endpoint, as it does for dev.idp
  - dev.otel.max bounds retained records per signal and zero keeps the viewer default
  - dev.logs only affects api:cli-dev and configures requirement:local-jsonl-log-capture
  - dev.logs.directory must be a relative directory within the project; it cannot name the project root or escape it
  - a missing dev.logs block keeps local JSONL capture enabled at .log, so existing projects acquire the developer convenience without runtime behavior changing
  - dev.console only affects api:cli-dev and configures requirement:dev-console
  - dev.console.port is fixed rather than reserved, because a console is bookmarked and returned to, and a reserved port would move every run
  - dev.console.enabled false disables every pane, including the requirement:dev-telemetry-viewer page, while dev.otel.enabled still governs whether records are received at all
  - a dev.console pane key disables one pane and nothing else, and a missing pane key means enabled
  - dev.console.overlay.reload only reloads a page requirement:dev-error-overlay is already attached to, and never restarts anything
  - dev.console.launcher.enabled serves requirement:dev-console-launcher, and it is independent of dev.console.overlay.enabled because a developer who wants the way in does not necessarily want a sheet over the page
  - dev.console.launcher.enabled and dev.console.overlay.enabled both false inject no console address into the application, which is what makes a served page byte-identical to a production render
  - dev.console.launcher.corner names one of the four corners and rejects any other value, as project.toolchain and project.database reject theirs; a silent fallback would leave a typo looking like a default the project chose
  - a missing dev.console.launcher.corner means bottom-left, which is the corner applications are least likely to have taken for themselves
  - dev.console.launcher.corner is read by the application process at startup, like the console address it travels with, so a changed corner arrives with the restart the edited file already causes
  - dev.console.storybook.enabled false builds and starts no decision:dev-harness-process binary, because nothing else needs it
  - dev.console.data.enabled serves the requirement:dev-data-pane browser and editor, and dev.console.queries.enabled the requirement:dev-query-runner halves on the same pane
  - both false leave the application serving no pane, so it announces nothing and decision:dev-application-attachment does not arise
  - relative paths resolve from the config file directory
  - unknown keys are errors
  - command flags override config values
  - missing config is an error except for api:cli-init
  - server, session, security, middleware, and observability runtime values are forbidden, and so is a database connection value; project.database names an engine, never a DSN or a credential
  - enabled Tailwind validates requirement:tailwind-css-integration and decision:tailwind-host-toolchain
  - Tailwind plugins and their options belong to the CSS entry through requirement:tailwind-plugin-integration
  - the CLI must already be available from the entered Devbox environment
runtime_configuration:
  owner: api:runtime-configuration
  file_selection: policy:config-file-resolution
  inputs:
    - TOML selected by configbind
    - environment
    - CLI flags
```
