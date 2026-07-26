---
id: api:cli-add
type: api
title: pw add
---
pw add installs a framework capability into an existing project, so a choice declined at api:cli-init is not a decision the project is stuck with.

```yaml
status: not implemented
usage: "pw add <capability> [capability flags]"
capabilities:
  auth:
    flags: "--mode=oidc, --devidp"
    writes:
      - rule:framework-owned-tables migration files for the session store and the authentication tables
      - data:authentication-runtime-config section in every environment configuration file
      - account resolver source that api:authentication-endpoints calls
      - data:devidp-config roster and data:project-config dev.idp when the local emulator is selected
    imports: the application already links plugin/auth through its account resolver, so no separate wiring step exists
versioning:
  migration_version: the next free version in the project migration directory
  rationale: an application that already applied 00001 through 00007 must not have those renumbered, so no version range is reserved for the framework
  identity: the name stem published by the owning package, which makes the file recognizable at any version
rules:
  - refuse a capability the project already carries, detected by migration name stem rather than by version
  - never rewrite or renumber an existing migration
  - never overwrite an application-owned file; report the conflict and stop
  - write nothing when any step would fail, so a partial capability cannot reach a project
  - print the commands that finish the installation, starting with api:migration-runner
  - reject a capability whose mode has no implementation, matching api:cli-init
relations:
  init: api:cli-init writes the same files for a new project
  layout: concept:project-layout
exit:
  success: 0
  already_present_or_conflict: nonzero with the path and the reason
```
