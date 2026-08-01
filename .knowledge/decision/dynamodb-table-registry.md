---
id: decision:dynamodb-table-registry
type: decision
title: Generated Table List In The Main Package
---
api:cli-generate registers every generated table constructor from the package that declares it, and system:pw-cli reads the assembled set through a framework action rather than a second on-disk artifact.

```yaml
status: accepted
problem: requirement:dynamodb-migration applies a desired state assembled from generated constructors, and neither the runtime nor system:pw-cli can enumerate them without being told
list:
  location: beside the generated codec, in the package that declares the type; the binary assembles the set from what it links
  form: an init registration beside the generated codec, calling api:dynamo-package RegisterTable with the declared name and the generated constructor
  revised: implementation 2026-08-01; the design called for an explicit slice in the main package, and init registration is what the rest of the framework already does
  why_init_won: every other capability here enters a binary the same way, from a session backend to a database engine, and one shape is worth more than the linker drop it costs
  cost: one table-definition constructor per tagged type is linked whether or not it is used; nothing links CreateTable unless a program calls it
  derived_from_the_generated_source: api:cli-generate reads the emitted table constructor rather than analyzing the package a second time, so a type whose constructor was suppressed registers nothing and needs no flag to say so
  declared_names: the snake_case of the Go type name, treating a run of capitals as one word, which is the same string a .pw.dynamo table clause and an item call use
  framework_entries: an imported framework package registers its own table the same way, which is how requirement:dynamodb-session-store reaches requirement:dynamodb-migration without a migration file
  import_driven: a framework table enters the desired state because its package was imported, so an unimported capability contributes no table and no plan entry
cli_transport:
  action: --pw-print-dynamo-schema, a framework-owned action beside the --pw-print-dsn of api:migration-runner
  behavior: parse configuration, resolve every deployed name through rule:dynamodb-table-naming, write the resolved table set to stdout, exit
  used_by: api:cli-migrate and api:cli-dev, run through the host Go toolchain
  precedent: the CLI already runs the application to learn the effective DSN instead of reimplementing configuration precedence
one_source_of_truth:
  fact: the in-process path and the CLI path read the same registered set through the same resolution
  rejected_alternative:
    form: a committed schema descriptor file written by api:cli-generate and read directly by the CLI
    attraction: the CLI would not need to build the application, and a destructive change would appear in a review diff
    why_not: it is a second artifact that can disagree with the generated Go, and the diff argument is answered by policy:dynamodb-migration-safety refusing destructive changes outright
    revisit_if: building the application to read a schema becomes the slow step of api:cli-dev
tinygo:
  fact: the client and the migrator both build under TinyGo, so a TinyGo binary answers the action itself
  contrast: decision:migration-execution-split exists because system:goose is host-only, and none of it applies here
related:
  - requirement:dynamodb-generation
  - requirement:dynamodb-migration
  - rule:dynamodb-table-naming
  - api:cli-migrate
```
