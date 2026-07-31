---
id: policy:dynamodb-migration-safety
type: policy
title: DynamoDB Migration Safety
---
Schema application for this store is additive; a destructive or unperformable change is reported and the run stops, and no flag turns that into an action.

```yaml
allowed_without_confirmation:
  - creating a table the account does not have
  - reporting a plan
refused_always:
  - deleting a table, whether or not the source still declares it
  - deleting a secondary index, once indexes become expressible
  - any action that would drop items
  form: the run reports the difference and exits nonzero without sending the request
  reason: DynamoDB has no transactional DDL and no rollback, so a destructive step is final; requirement:database-migration can confirm a down because a SQL rollback is a documented reverse statement, and this has none
  no_escape_flag: no --allow-destructive; an operator who intends a deletion performs it with an AWS tool, where the account's own guard rails apply
unperformable:
  cases:
    - a partition key, sort key, or key attribute type that differs from the deployed table
    - a billing mode or capacity that differs, which the driver cannot change
  form: an error naming the table, the attribute, the desired value, and the observed one
  remedy: stated in the message as an operator action, since a rename or a rebuild is not this mechanism's to perform
automatic_apply:
  development: api:cli-dev applies, matching the SQL path
  test: requirement:dynamodb-test-isolation applies into its own prefixed table set
  startup:
    default: disabled
    enable: data:dynamodb-runtime-config auto_migrate true, explicitly set
    scope: create only, which is the whole of what apply can do
    reason_it_is_safer_than_the_sql_case: create is idempotent and additive, and every other change is refused before a request is sent
concurrency:
  - a duplicate CreateTable returns ErrTableInUse, which is read as already applied rather than as a failure
  - no advisory lock exists or is needed, because the only mutation is idempotent
  - two instances starting together converge on the same table set
credentials:
  - never passed as a process argument, including to the decision:dynamodb-table-registry framework action
  - redacted from output, logs, errors, and policy:startup-summary
observability:
  - log each created table with its resolved physical name and elapsed time
  - report the full plan before applying, so an operator sees what a run will do
  - report a refused change at the same level as a failure, never as a warning that a run continues past
```
