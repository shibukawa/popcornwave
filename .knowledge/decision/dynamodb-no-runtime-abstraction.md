---
id: decision:dynamodb-no-runtime-abstraction
type: decision
title: No pw Or pwruntime Abstraction For DynamoDB
---
Popcorn Wave configures and owns the DynamoDB client and stops there; api:dynamo-package is the whole surface, and neither pw nor pwruntime gains a DynamoDB symbol.

```yaml
status: accepted
decided: user 2026-07-31
reason:
  no_common_interface: database/sql is what lets pw hide three SQL engines behind one executor; DynamoDB has no such interface to hide behind, so a wrapper would abstract exactly one implementation
  generated_code_needs_nothing: dynamobind helpers take a *dynamodb.Client argument, so there is no executor resolver to install and nothing for pwruntime to answer
  passthrough: system:tinybind commits dynamobind to passing driver errors, retries, and page boundaries through untouched, and a pw facade would be the layer that breaks it
what_pw_still_owns:
  - reading data:dynamodb-runtime-config and constructing the client
  - installing the client into data:request-context-capsule
  - closing it through api:application-lifecycle
  - the table name resolution of rule:dynamodb-table-naming
  - requirement:dynamodb-migration
  reason: these are lifecycle and configuration, which the application should not repeat, and none of them wraps a driver call
what_pw_does_not_own:
  - a Load, Store, Query, or Scan wrapper; the application calls dynamobind directly
  - an error type over the driver sentinels
  - a pwruntime executor resolver, connection group, or transaction scope
  - a pw.SelectDynamo mirroring api:database-selection
boundary_effects:
  concept:public-package-boundaries: database/dynamo joins the public lower-level list and is deliberately absent from the pw surface
  import_style: a normal import, not a blank one; the same import both registers the configuration binding and provides the accessor
  contrast_with_rdb: an rdb engine is blank-imported because the application never names it, while an application using DynamoDB names this package on every call
upstream_removed_the_pressure:
  was: a generated query took the client and the table name, so a thin wrapper filling those two arguments looked tempting
  now: system:tinybind carries the client in the context and resolves the table inside its own runtime entries, so a declared query takes context and parameters alone
  effect: the wrapper that was being considered would have exactly nothing left to do
  better_than_asked: the request from here was a generation option selecting a framework resolver, mirroring the SQL path; moving resolution into the runtime removes the generated call site a framework would have redirected, so no seam is configured at all
  what_pw_installs: the client and the name resolver, once, through the system:tinybind client setter in the api:dynamo-package middleware
accepted_cost:
  visibility: an application handler imports the binding package and the driver, which the SQL path would have let it skip
  judged: acceptable, and now the only cost; the alternative is a facade that renames every driver symbol and still cannot remove either import from generated code
  binary: the context client path costs about 37 KB on a TinyGo wasip1 build, measured upstream, from the context value and the assertion that reads it back
  escape: a size-critical program calls the driver directly with the generated methods and links none of it
remaining_argument:
  what: an item operation still names a table, having no declaration to read one from
  not_a_gap: it is the absence of a declaration rather than an inconsistency, and a wrapper hiding it would have to invent which table a type belongs to
revisit_if:
  - a second document store arrives that could share one interface
  - an item operation gains a declaration form, which would leave nothing named at any call site
related:
  - requirement:dynamodb-store
  - api:dynamo-package
  - concept:public-package-boundaries
```
