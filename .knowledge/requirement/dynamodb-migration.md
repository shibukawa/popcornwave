---
id: requirement:dynamodb-migration
type: requirement
title: DynamoDB Schema Application
---
The desired table set generated from dynamo tags is applied against the live account, creating what is missing and refusing to reshape what exists.

```yaml
model: decision:dynamodb-desired-state-migration
desired_state: decision:dynamodb-table-registry, resolved through rule:dynamodb-table-naming
observed_state: DescribeTable per resolved name, per system:tinygodriver-dynamodb
safety: policy:dynamodb-migration-safety
flow: flow:dynamodb-migration
surfaces:
  cli: api:cli-migrate, dispatching to this store when data:dynamodb-runtime-config is enabled
  application: api:dynamo-package Migrate and Plan
  development: api:cli-dev automatic apply, the same step that applies pending SQL migrations
  test: api:test-run, per requirement:dynamodb-test-isolation
  parity: the same CLI verbs and the same runner shape as requirement:database-migration, which is what makes this one mechanism rather than two
change_kinds:
  create:
    trigger: DescribeTable reports ErrTableNotFound
    action: CreateTable with the generated keys and nothing else
    wait: poll DescribeTable until the table is active, because the driver ships no waiter
  verify:
    trigger: the table exists
    action: compare the partition key, the sort key, and their attribute types, and nothing else
    match: no request is sent
    mismatch: an error naming the table, the attribute, the desired shape, and the observed one
    deliberately_out_of_the_comparison: everything decision:dynamodb-operational-configuration assigns to deployment tooling
  alter: not expressible; the driver has no UpdateTable, so a key change is reported rather than performed
  delete: never, per policy:dynamodb-migration-safety
forward_only:
  fact: no down, down-to, or rollback action exists for this store
  reason: reversing a DynamoDB change is DeleteTable, which destroys data that no SQL DDL rollback destroys
  correcting: a mistaken table is removed by an operator, deliberately and outside this mechanism
no_version_table:
  reason: DescribeTable reports the live shape, so the state a goose_db_version table exists to remember is directly observable
  consequence: repeated apply is a no-op by construction rather than by bookkeeping, and there is nothing to get out of step with the source
capacity_and_billing:
  configured: nowhere; the keys were removed once creation became a development step, per data:dynamodb-runtime-config
  created_with: the driver default of on-demand billing, which is what a local emulator ignores anyway
  never_compared: a deployed table's billing mode and capacity are not read, not reported, and not corrected
  why_not_even_reported: a correct production table is provisioned by deployment tooling, so reporting the difference would fire on every correct deployment and train a reader to ignore the report
what_desired_state_means_here:
  covers: table existence and key schema, per decision:dynamodb-operational-configuration
  excludes: TTL, retention, autoscaling, tags, and replication, which deployment tooling defines
  effect: this mechanism does not claim to apply everything about a table, and says which half it applies
  reason_it_is_not_a_shortfall: a production table has an owner already, and two authors of one resource is worse than one narrow one
creation_is_for_development:
  decided: user 2026-08-01
  create: development and test, where nobody wants to run deployment tooling to get a table
  production: the table comes from deployment tooling; this mechanism verifies and reports, and creates nothing at startup
  why_this_differs_from_requirement:database-migration:
    relational: a schema is normally outside what infrastructure tooling manages, so versioned migrations are the mature answer and own production
    dynamodb: a table reads as part of the infrastructure, so the same tooling that creates the queue and the bucket creates it too
    consequence: the same desired-state comparison serves as an authoring step in development and as a check in production
  publishing: the definitions are printable, so deployment tooling copies what the code declares instead of restating it, per api:cli-migrate
  verification_is_the_production_value:
    what: the deployed key schema differs from what the generated code expects
    why_it_needs_the_framework: deployment tooling knows what it created and not what the application assumes, so only the application can compare the two
    when: startup, per rule:framework-owned-tables, and on demand through api:cli-migrate
empty_schema_source:
  trigger: the item-table feature suppressed, per requirement:dynamodb-generation
  behavior: report that no table definition is generated, not that every table is missing
  reason: a project managing tables with Terraform has made a choice, and a plan proposing to create all of them would be wrong rather than merely noisy
acceptance:
  - a first development run against an empty account creates every generated table and returns active
  - an application whose deployed table has a different partition key refuses to start, naming both shapes
  - auto_migrate outside development fails configuration load rather than creating a table
  - the printed definitions round trip through deployment tooling into a table the same build then verifies as matching
  - a second run sends no write request and reports no change
  - a table whose partition key differs from the generated one fails with both shapes named and creates nothing
  - a generated table added to the source appears as one pending create in Plan before it is applied
  - a table present in the account and absent from the source is reported and never deleted
  - the CLI, the application entry point, and a test produce the same plan for the same configuration
  - credentials appear in no output, log, error, or process argument
non_goals:
  - versioned migration files for this store; the generated schema is the source
  - data backfill or item transformation
  - secondary index management, until system:tinybind ships the gsi tag option and the driver ships UpdateTable
  - TTL, autoscaling, tags, backup, or global tables, none of which the driver exposes
  - coordinating concurrent migrators; DynamoDB rejects a duplicate CreateTable with ErrTableInUse, which is treated as already applied
```
