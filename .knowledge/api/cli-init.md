---
id: api:cli-init
type: api
title: pw init
---
pw init creates a runnable Popcorn Wave project with a shared document shell, representative handler, typed page template, SQL query, error pages, Devbox environment, and generated-artifact conventions.

```yaml
usage: pw init [myapp] [--interactive] [--tailwind|--no-tailwind] [--tinygo|--no-tinygo] [--auth=none|oidc|oidc-passkey|passkey] [--devidp|--no-devidp]
mode: decision:interactive-project-bootstrap
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
  authentication:
    default: none
    none: no data:authentication-runtime-config section is written
    oidc: auth.mode oidc
    oidc-passkey: auth.mode oidc_passkey per decision:authentication-bootstrap-strategy, with recovery.policy oidc
    passkey: auth.mode passkey_only, with registration.policy and recovery.policy both administrator and the bootstrap bounds set
    passkey_scaffold:
      when: the selected mode mounts api:passkey-endpoints
      config: passkey.rp_id localhost, passkey.origins the development origin, user_verification required, discoverable preferred
      origin: an OIDC redirect_url in a passkey mode uses localhost rather than 127.0.0.1, because an origin has to sit inside the RP ID and an address can never be one
      accounts: SetAccountLookup for every passkey mode, plus SetAccountActivator and an IssueBootstrapCredential wrapper for passkey_only
      browser: public/passkey.js, dependency free, because the framework serves the endpoints but cannot call navigator.credentials for the page
      page: controls bound by element id, so the template carries no inline script
      emulator: refused outside an OIDC mode, so passkey_only never scaffolds an identity provider roster
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
  - flow:handler-registration mux for the selected toolchain
  - handler registration and pw.Parse example
  - templates/document.pw.html shared document shell
  - .pw.html page and 400, 401, 403, 404, 409, 413, and 500 templates
  - .pw.sql query example
  - migrations/00001_init.sql application schema as migration version 1
  - public directory with non-served .keep sentinel and stable public.go embedding scaffold
  - tinygohelper.go netdev registration for rule:tinygo-runtime-compatibility, only when TinyGo is selected
  - .gitignore excluding **/*_pw_gen.go generated application build inputs
  - .vscode/settings.json hiding **/*_pw_gen.go
  - Devbox configuration with Valkey enabled by default and TinyGo when selected
  - data:authentication-runtime-config section for the selected authentication mode
  - data:devidp-config roster and data:project-config dev.idp when the local emulator is selected
  - api:authentication-endpoints blank import in main and a sign-in and sign-out control on the starter page
  - rule:framework-owned-tables migrations from the packages that own those tables, at the versions after the application schema, when the mode serves a login
  - data:middleware-runtime-config rdb settings, because the scaffolded migrations and queries need a database
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
next_steps:
  - cd myapp
  - devbox shell
  - pw dev
exit:
  success: 0
  wizard_canceled: 0 with a canceled notice and no files written
  invalid_input_or_collision: nonzero with actionable path
```
