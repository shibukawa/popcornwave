---
id: api:cli-package
type: api
title: Package Authoring Commands
---
The CLI gains a package project kind, so a concept:component-package is scaffolded, generated, and checked by the same commands an application uses.

```yaml
scaffolding:
  command: "pw init --kind package"
  writes:
    - data:project-config with project.kind = "package", no project.main, and the data:component-package-manifest package section
    - go.mod, .editorconfig, and .vscode settings, unchanged from the api:cli-init application scaffold
    - .gitignore without the **/*_pw_gen.go rule, per decision:committed-package-artifacts
    - a starter component or handler for the shape being authored
  omits:
    - concept:application-entry-point and its generated registration linker
    - a requirement:nested-html-templates document shell and the error pages, which belong to the consuming application
    - environment configuration files
    - devbox.json, unless the package's own tests need a service
generation:
  command: api:cli-generate, unchanged
  differences_in_package_kind:
    - no generate.config fixed path, because there is no main package to receive the registration linker
    - no document shell lookup, so a generate.templates entry holding one is an error instead of the project's shell
    - no generate.queries entry may be non-empty, per the requirement:component-package-distribution non-goal
    - the emitted artifacts are the same in every other respect, which is what makes a package's output linkable
  check: api:cli-generate --check is the package's release gate, per decision:committed-package-artifacts
consuming:
  command: "pw add <module-path>"
  writes: the go.mod requirement and one entry in the data:project-config packages array, and nothing else
  optional: the command is a convenience; adding both lines by hand is a supported install, per decision:declared-package-installation
  no_wizard: there is nothing to review, because nothing is copied; a package needing a capability the project lacks is reported and the built-in wizard is offered for that capability alone
  prints: the Register call, for a package whose manifest names one, because concept:application-entry-point is application-owned
  behavior: flow:component-package-consumption
  relation: api:cli-add gains a module argument beside its capability argument; the capability path still writes files, per rule:framework-owned-tables
linking:
  where: the generated bootstrap of the project.main package, which already exists as the registration linker
  what: one blank import per declared package, emitted by api:cli-generate
  effect: the declaration is the install, and removing the line removes the import on the next generation
diagnosis:
  command: api:cli-doctor
  added_checks:
    - every declared package, with its declared and resolved versions
    - a policy:package-compatibility version outside the supported window
    - a declared package whose requirement:package-schema-contribution stream has never been applied
    - a declared module missing from go.mod, and a linked package the project never declared
    - a package section in an application project, or project.main in a package project
    - tables left by a declaration that was removed
development:
  api:cli-dev: unavailable in a package project, because there is nothing to run; the package's own tests are the loop
  api:cli-build: unavailable for the same reason, and the release gate is the check above
rules:
  - every command reads project.kind first and reports a command that does not apply to the kind rather than failing later
  - a package project is otherwise an ordinary project, so api:cli-new adds sources into it under the same purposes
  - nothing about an application project changes; project.kind defaults to application and a file without the key is an application
```
