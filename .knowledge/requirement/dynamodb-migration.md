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
    action: CreateTable with the generated keys and the configured billing mode and capacity
    wait: poll DescribeTable until the table is active, because the driver ships no waiter
  verify:
    trigger: the table exists
    action: compare the partition key, the sort key, and their attribute types
    match: no request is sent
    mismatch: an error naming the table, the attribute, the desired shape, and the observed one
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
  source: data:dynamodb-runtime-config, not the struct tags
  applies: at create time only, because the driver cannot change them afterwards
  drift: a billing mode changed outside this mechanism is reported by Plan and not corrected
ttl:
  today: absent from the applied state, because the driver exposes no UpdateTimeToLive
  consequence: an expiring record needs an operator to enable expiry by hand, which must be documented rather than implied, since a TTL attribute nothing acts on looks like a working feature and silently retains every record
  when_the_driver_gains_it: a ttl tag reaches the generated table definition, and expiry joins the create step as one more field rather than as a workflow
  why_it_matters: this mechanism claims to apply a table's desired state, and cannot fully claim it while TTL sits outside what it can apply
empty_schema_source:
  trigger: the item-table feature suppressed, per requirement:dynamodb-generation
  behavior: report that no table definition is generated, not that every table is missing
  reason: a project managing tables with Terraform has made a choice, and a plan proposing to create all of them would be wrong rather than merely noisy
acceptance:
  - a first run against an empty account creates every generated table and returns active
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
