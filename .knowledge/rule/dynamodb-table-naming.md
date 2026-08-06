---
id: rule:dynamodb-table-naming
type: rule
title: DynamoDB Table Name Resolution
---
Source declares a table name; a resolver installed once maps it onto the deployed one, and nothing else in the project builds a table string.

```yaml
mechanism:
  owner: system:tinybind, which takes a resolver function as a handle option and runs it inside every runtime entry
  signature: "func(ctx context.Context, declared string) string"
  installed_by: api:dynamo-package setup, once per process, as an option of the dynamobind Handle it builds; no per-request installation exists, per requirement:context-lookup-performance
  absent: the declared name is sent unchanged, so a deployment named as declared configures nothing
declared_name:
  query: the table clause of a .pw.dynamo statement, required in every declaration, per requirement:dynamodb-typed-queries
  item_operation: the argument a call passes, since it has no declaration to read one from
  per_type_default: the snake_case of the Go type name, recorded by decision:dynamodb-table-registry, which is what an item call and the generated table constructor use
  check: a table clause naming a table with no generated definition is a generation error, so the read path and requirement:dynamodb-migration cannot disagree about which tables exist
why_not_a_prefix_alone:
  fact: a DynamoDB table name has no structure the service reads, unlike an S3 key prefix that listing, IAM, and lifecycle rules all understand
  cannot_express: a CDK generated physical name carrying a suffix, an "orders-prod" that puts the environment last, or a name read from an environment variable that shares nothing with the declared one
  consequence: the framework configures a function, and a prefix is one thing that function can be
configuration:
  table_prefix: prepended to the declared name; the common case, and what requirement:dynamodb-test-isolation uses
  table_names: an explicit declared-to-deployed map, for a name no prefix produces
  precedence: an explicit entry wins, otherwise the prefix is applied, otherwise the declared name stands
  escape: api:dynamo-package WithTableResolver replaces the composed function entirely, for a deployment neither key expresses
one_resolver:
  rule: the request path and requirement:dynamodb-migration use the same function, built from data:dynamodb-runtime-config
  reason: a migration that creates a table a handler cannot find is the failure this rule exists to prevent
  no_request_context: the CLI has no request, so the function must be constructible from configuration alone; a resolver depending on request state is an application's own and is outside what the framework installs
per_request_naming:
  possible: the resolver takes a context, so a per-tenant table is expressible
  framework_position: not configured here; a project needing it installs its own function and owns the consequence for migration, which cannot enumerate tenants
validation:
  - a declared name matches the DynamoDB rule of three to 255 characters of letters, digits, underscore, hyphen, and dot, checked at generation for a declaration and at startup for a configured mapping
  - a resolved name satisfies the same rule, checked at startup rather than at the first request
  - a table_names entry naming an undeclared table is a configuration error, since it would silently do nothing
not_covered:
  - renaming a deployed table; the driver has no rename and requirement:dynamodb-migration will not delete one
  - a resolver that maps two declared names onto one deployed table, which the framework neither prevents nor supports
related:
  - api:dynamo-package
  - data:dynamodb-runtime-config
  - decision:dynamodb-table-registry
  - requirement:dynamodb-typed-queries
```
