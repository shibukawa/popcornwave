---
id: decision:dynamodb-desired-state-migration
type: decision
title: Desired State Instead Of Versioned Migrations
---
The DynamoDB schema is the set of table definitions generated from dynamo tags, applied by comparison against the live account, with no migration files and no version table.

```yaml
status: accepted
decided: user 2026-07-31
why_not_goose: system:goose is a SQL engine and host-only; neither its dialects nor decision:migration-execution-split reach this store
alternatives_considered:
  versioned_declarative_files:
    form: numbered YAML or TOML files under a directory, mirroring data:migration-source
    for: it reuses the immutable, committed, reviewable convention the SQL path already teaches
    against: it restates the key names the tags already carry, which is exactly the drift requirement:dynamodb-store consumes dynamobind to remove
    rejected: yes
  go_function_migrations:
    against: decision:goose-migration-engine restricts the SQL path to SQL so host and TinyGo stay equivalent, and an arbitrary Go step reintroduces the divergence it removed
    rejected: yes
  chosen: the generated table definitions as the desired state
    for:
      - the key names in the codec, the key builder, the schema, and the request come from one declaration
      - DynamoDB is introspectable through DescribeTable, so the comparison needs no recorded history
      - a schema this small has no ordering problem for a version sequence to solve
why_no_version_table:
  sql_reason_absent: goose records versions because a SQL schema cannot be reliably read back into the statements that produced it
  here: a table is a partition key, an optional sort key, and their types, all reported by DescribeTable
  effect: the plan is derived from what is actually deployed rather than from what a table claims was applied, so a hand-made change is visible rather than hidden
the_risk_this_creates:
  statement: a desired-state applier can act on a deletion that was only a source edit
  today: not reachable; the only expressible changes are create and verify, because the driver has no UpdateTable and the tags carry no secondary index
  guarded_by: policy:dynamodb-migration-safety, which refuses every destructive change rather than confirming it
when_update_table_lands:
  paired_with: secondary index tags in system:tinybind; either change alone is worth little, because UpdateTable alone can only correct configuration drift and index tags alone produce a definition nothing can apply
  ranking: system:tinygodriver-dynamodb places UpdateTable second, behind UpdateTimeToLive
  still_unperformable: the partition key, the sort key, their attribute types, and every local secondary index stay immutable, so the policy:dynamodb-migration-safety unperformable category shrinks rather than disappears
  becomes_a_sequenced_plan:
    - one index operation per UpdateTable call, so a plan touching several is applied in order
    - each step waits for the index to become active before the next, since the driver ships no waiter
    - creating an index on a populated table backfills, which can take far longer than a schema statement ever does
  blocking_question:
    decided: start the change and report it; do not wait for a backfill to finish
    reason: blocking would make api:cli-dev unable to start, and the mechanism stays idempotent either way because a re-run reports an in-progress index as in progress
    surface: api:cli-migrate status reports a creating index, which is the state a version table would otherwise have had to remember
  index_deletion: refused, per policy:dynamodb-migration-safety, which already states that an index disappearing from a struct is reported and never applied
  capacity_correction:
    possible: yes, once UpdateTable exists
    rule: only under an explicit api:cli-migrate run, never under the data:dynamodb-runtime-config auto_migrate startup path
    reason: writing a configured provisioned capacity back on every boot fights the account's own autoscaling
scope_of_the_word_migration:
  kept: the api:cli-migrate verbs, the runner shape, and the api:cli-dev apply step, because the operator experience should not fork
  dropped: version numbers, files, up-by-one, up-to, down, down-to, and create
related:
  - requirement:dynamodb-migration
  - policy:dynamodb-migration-safety
  - decision:dynamodb-table-registry
  - requirement:database-migration
```
