---
id: decision:declared-package-installation
type: decision
title: Declared Package Installation
---
Installing a concept:component-package is one declaration in data:project-config, plus a Register call when it serves routes; nothing is copied into the project tree.

```yaml
status: accepted
question: whether api:cli-add installs a package by writing its migrations and configuration into the project, the way it installs a built-in capability, or whether the project only names the module
chosen: name the module
declaration:
  form: a packages array of tables in data:project-config, one entry per module the application uses
  minimum: the module path
  effect: api:cli-generate emits the blank import into the generated bootstrap, so the declaration is what links a package the application never names in Go
  redundant_when: the application imports the package to call its Register, which already links it; the generated blank import is then harmless duplication rather than the mechanism
  still_required_then: the declaration is what api:cli-migrate and api:cli-doctor read, and neither can parse the main package to learn what it imported
why_a_declaration_and_not_go_mod:
  go_mod_says: the module is available, including every transitive dependency the application never asked for
  the_list_says: the application intends to use it, which is a different fact and the one every consumer of it needs
  generation: the bootstrap generator must know which modules to import, and cannot infer intent from a dependency graph
  consistency: requirement:component-package-distribution already states that no contribution is discovered by scanning
per_contribution:
  configuration:
    copied_before: the package's section appended to every environment file
    declared_now: nothing; the package's generated pw.RegisterConfig binding registers from init and its defaults apply, exactly as a framework capability's do
    finding: the copy was never required. api:runtime-configuration resolves values from registered definitions, and a section in an environment file is an override an operator writes when they want one
    tooling: api:cli-generate can still print or scaffold the section on request, because showing an operator which keys exist is worth doing and writing them is not
  routes:
    copied_before: nothing; the Register call was printed as a manual step
    declared_now: unchanged; the application still calls the package's Register on its own mux
    considered: an init-registered route contributor installed by pw.NewServeMux, which would have made the declaration complete for a route-serving package too
    rejected: it is the framework route registration API api:framework-extension states does not exist, and mounting is the one contribution an application has an opinion about
    consequence: a route-serving package is one declaration plus one call, and this decision does not claim otherwise
    boundary_held: no framework surface is added for routes, which keeps this decision entirely about removing work rather than adding a mechanism
  migrations:
    copied_before: the package's files copied into the project migration directory at the next free version
    declared_now: the package keeps its own migration stream, per requirement:package-schema-contribution
    finding: the copy was defensible and is not required; it bought reviewability and cost an upgrade step that nothing forces anyone to run
  assets: never copied; requirement:package-asset-delivery serves them from embedded bytes
what_this_costs:
  schema_visibility: the project repository no longer contains the SQL that runs against its database, which the copy model did give it
  mitigation: api:cli-migrate reports pending package migrations with their statements before applying, go.sum pins the bytes, and the source is readable in the module cache
  operator_edits: an operator cannot tune a package's migration; they add their own migration afterwards, exactly as they cannot edit a dependency's Go
  accepted_because: a copy that must be re-run after every go get -u is a step whose omission is silent, and silence about schema is worse than opacity about it
rejected_alternative:
  copy_into_the_project:
    kept_for: rule:framework-owned-tables, which is unchanged; a framework capability is installed once by a wizard the operator ran, and has no module version that moves underneath it
    why_not_here: a package version moves with go get -u, and an install model that needs a second command to stay correct will drift
    honest_advantage_given_up: every statement that touches the database is visible in one repository and reviewed in the project that runs it
uninstall:
  form: remove the declaration; the package stops being imported, its routes and configuration disappear, and its tables stay
  tables: dropping them is not offered, matching the requirement:incremental-project-capabilities non-goal
open_questions:
  - whether a package may be declared and disabled, or whether removing the line is the only off switch
resolved_questions:
  mount_prefix: not a question here any more; the application calls Register and passes whatever the package's own surface accepts, so a prefix is the package's API rather than a declaration key
```
