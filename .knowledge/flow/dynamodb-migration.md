---
id: flow:dynamodb-migration
type: flow
title: DynamoDB Schema Application
---
Resolve the generated table set, read the account, plan, then create only what is missing.

```yaml
trigger:
  - api:cli-migrate against a project whose data:dynamodb-runtime-config is enabled
  - api:cli-dev before the application accepts requests
  - api:dynamo-package at startup, verifying by default and creating only when auto_migrate is set in development
  - api:test-run per requirement:dynamodb-test-isolation
steps:
  - id: resolve
    do: read the generated list of decision:dynamodb-table-registry and build each definition with its physical name from rule:dynamodb-table-naming
    add: the billing mode and capacity of data:dynamodb-runtime-config
  - id: observe
    do: DescribeTable for every resolved name
    outcomes:
      ErrTableNotFound: the table is missing
      description: the deployed keys and their attribute types
      other_error: stop, naming the table with credentials redacted
  - id: plan
    do: classify each table as create, match, or mismatch
    also: list a deployed table that no generated definition claims, as a report-only entry
  - id: gate
    do: stop before any request when the plan holds a mismatch or a refusal, per policy:dynamodb-migration-safety
    output: every offending table in one report, not only the first, so an operator sees the whole problem
  - id: apply
    do: CreateTable for each missing table
    only_when: the run is allowed to create, which policy:dynamodb-migration-safety limits to development, test, and an explicit operator command
    otherwise: stop after the plan, reporting a missing table as a failure rather than creating it
    idempotent: ErrTableInUse is read as already created
    then: poll DescribeTable until active, since the driver ships no waiter
  - id: report
    do: return the created tables and the unchanged ones, and log each creation with its elapsed time
paths:
  cli: system:pw-cli obtains the resolved set through the decision:dynamodb-table-registry framework action, then runs the same steps
  in_process: api:dynamo-package reads the list directly and runs the same steps
  guarantee: one implementation, so both report the same plan for the same configuration
plan_only: api:dynamo-package Plan and the api:cli-migrate dry run stop after the gate step
no_delegation: both host Go and TinyGo run this in process, unlike decision:migration-execution-split
errors:
  - a missing region or credential, reported before the first request
  - a key shape that differs from the deployed table
  - a physical name exceeding the DynamoDB length limit, reported at resolve rather than at create
  - a create that fails, leaving already-created tables in place because there is nothing to roll back
```
