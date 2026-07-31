---
id: decision:dynamodb-table-registry
type: decision
title: Generated Table List In The Main Package
---
api:cli-generate collects every generated table constructor into one explicit list in the project.main package, and system:pw-cli reads it through a framework action rather than a second on-disk artifact.

```yaml
status: accepted
problem: requirement:dynamodb-migration applies a desired state assembled from generated constructors, and neither the runtime nor system:pw-cli can enumerate them without being told
list:
  location: the project.main directory, the fixed path that already receives the generated registration linker
  form: an explicit slice of declared name and constructor pairs, referenced by api:dynamo-package Migrate and Plan
  declared_names: the same strings a .pw.dynamo table clause and an item call use, so requirement:dynamodb-typed-queries can be checked against this set at generation
  not_init: no init registration, so the static dispatch of system:tinybind keeps its property that an unused type links nothing
  cost: a binary exposing the framework action links every constructor, which is one leaf function per type; nothing links CreateTable unless the program calls it
cli_transport:
  action: --pw-print-dynamo-schema, a framework-owned action beside the --pw-print-dsn of api:migration-runner
  behavior: parse configuration, resolve every physical name through rule:dynamodb-table-naming, apply the data:dynamodb-runtime-config billing and capacity values, write the resolved table set to stdout, exit
  used_by: api:cli-migrate and api:cli-dev, run through the host Go toolchain
  precedent: the CLI already runs the application to learn the effective DSN instead of reimplementing configuration precedence
one_source_of_truth:
  fact: the in-process path and the CLI path read the same generated list through the same resolution
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
