---
id: api:cli-dev
type: api
title: pw dev
---
pw dev runs the local service set and continuously regenerates, rebuilds, and restarts the configured application.

```yaml
usage: pw dev
steps:
  - start configured Devbox services
  - run api:cli-generate
  - run api:cli-migrate up when data:project-config migration.auto is enabled
  - start requirement:contrib-devidp when data:project-config dev.idp.enabled is true
  - start requirement:dev-telemetry-viewer unless data:project-config dev.otel disables it
  - start flow:tailwind-css-build watch mode when enabled
  - enable decision:development-public-assets
  - build and run data:project-config project.main
  - default data:runtime-environment to dev when APP_ENV is unset
  - watch every Go, .pw.html, .pw.sql, popcornwave.toml, config.*.toml, and config/config.*.toml source
  - watch the data:devidp-config file when the development identity provider is enabled
  - watch the data:migration-source directory
  - add data:project-config dev.extra_watch paths and globs
  - exclude public/** and public/**/*.zstd from Go rebuild inputs
  - regenerate when generated inputs change
  - reapply pending migrations before restart when migration sources changed
  - rebuild and restart after successful changes
services:
  default: Valkey
  rule: default services may be disabled or changed in Devbox configuration
  none_declared:
    behavior: skip startup silently when devbox reports that the project defines no service
    rationale: a project with no database and no cache is an ordinary shape, and starting anyway prints an error that reads like a misconfiguration
    probe: devbox exits zero either way, so the wording of its listing is the only signal; an unreadable answer falls through to starting services rather than skipping them
  output: service logs join the developer loop stream, because the process manager terminal UI would paint over generation, migration, and application output
  lifetime: services stop with the developer loop, because interrupting devbox leaves the process manager it spawned running
identity_provider:
  package: requirement:contrib-devidp
  lifetime: starts before the application process and stops with the developer loop
  issuer:
    port: an available loopback port reserved per run, because the injected issuer makes a fixed port unnecessary
    pin: data:project-config dev.idp.port pins it only when an external client needs a stable registration
  client:
    ownership: pw dev registers one ephemeral client per run and the developer declares none
    credentials: client id and secret generated from crypto/rand at startup and discarded at shutdown
    redirect: loopback redirect URIs accepted per policy:devidp-safety, so the application callback path stays application-owned
  injection:
    mechanism: environment variables on the application process, which outrank TOML in data:loaded-configuration precedence
    variables: data:authentication-runtime-config oidc issuer, client id, and client secret environment names
    rule: injected only for the process api:cli-dev starts, never exported to the developer shell
    override: an explicit environment value already set by the developer is preserved
  reporting: log the issuer, the login URL, the roster subjects, and the client id, with the secret masked
  reload: roster changes reload in place without restarting the application
  guardrails: policy:devidp-safety
  default: disabled
telemetry_viewer:
  requirement: requirement:dev-telemetry-viewer
  flow: flow:dev-telemetry-capture
  lifetime: starts before the application process, survives every rebuild and restart, and stops with the developer loop
  injection: data:observability-runtime-config otel enabled and endpoint, on the application process only
  output: records reach both the viewer and the developer loop stream, because a viewer must not empty the terminal
  default: enabled
migration:
  default: enabled and forward-only under policy:migration-safety
  ordering: migrations complete before the application process starts
failure:
  generation_css_or_build: keep the developer loop alive and report diagnostics
  migration: report diagnostics, skip the restart, and keep the developer loop alive
  identity_provider: report diagnostics and stop the loop, because the application cannot log in without its configured issuer
  telemetry_viewer: report diagnostics and keep the loop alive, because an unobservable run is still a working one
```
