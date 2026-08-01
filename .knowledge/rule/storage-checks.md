---
id: rule:storage-checks
type: rule
title: Storage Checks
---
The PW03xx data:diagnostic-check entries: whether the migration sources are well-formed, and, when contacting the database is permitted, whether the database still matches them.

```yaml
offline:
  inputs: data:migration-source files and the generated query artifacts only
  migration-version-gap:
    trigger: a gap in the version sequence of data:migration-source
    severity: note
    reason: goose applies lexically and tolerates gaps, so this is a merge artifact worth seeing, not a fault
  duplicate-migration-version:
    trigger: two sources sharing one version
    severity: error
    relation: the condition pw migrate validate reports, run here without a database
  migration-parse-failure:
    trigger: a source goose cannot parse or order
    severity: error
  local-database-path-unwritable:
    trigger: a sqlite file DSN whose parent directory does not exist or is not writable
    severity: error
    reason: the file is created on first open, so the failure otherwise appears at startup as a driver error with no path in it
  applied-state-unknown:
    trigger: any run without the online gate, when the database capability is present
    severity: note
    message: names the pending-count question as unanswered and points at pw migrate status
    reason: an unanswered question stated is better than a section that looks clean
online:
  gate: the api:cli-doctor online option, because every check here connects to the configured database
  engine: the goose and driver linkage api:cli-migrate already carries in the pw binary, so no application build is involved
  connection-failed:
    trigger: a configured connection that does not accept a bounded connect and ping
    severity: error
    evidence: the data:database-connection-set group#ordinal label and the redacted DSN, never the credential
    absent_local_file: a sqlite file that does not exist yet is reported as absent rather than opened, because opening it would create it, and a diagnosis that writes is not one
  pending-migrations:
    trigger: applied state behind data:migration-source
    severity: warning
    emphasis: data:project-config migration.auto false means nothing applies them during api:cli-dev either, which is the case that surprises people
  schema-drift-from-sources:
    trigger: the live schema differs from the data:migration-snapshot the same sources produce in a throwaway database
    severity: warning, and error when the difference removes something the generated queries use
    catches: manual DDL, a migration edited after it was applied, and a database restored from an older state
    reason: goose records applied versions and not checksums, so comparing the produced schema is what makes an edited migration visible at all
  generated-query-schema-mismatch:
    trigger: a table or column the generated .pw.sql artifacts reference that the live schema does not have
    severity: error
    reason: it compiles, so nothing before the first request says otherwise
dynamodb:
  applies_when: data:dynamodb-runtime-config is enabled
  inputs_offline: the decision:dynamodb-table-registry definitions and the generation sources, with no network
  offline:
    dynamo-purpose-without-sources:
      trigger: a data:project-config generate.dynamo entry whose directory holds no tagged type and no .pw.dynamo declaration
      severity: note
      reason: it is how a renamed directory presents, and the purpose then silently generates nothing
    dynamo-tag-error:
      trigger: a tag or declaration system:tinybind rejects, reported without writing a file
      severity: error
      relation: the condition api:cli-generate reports, run here without generating
    dynamo-name-too-long:
      trigger: a declared name plus the rule:dynamodb-table-naming mapping exceeding the DynamoDB length limit
      severity: error
      reason: it fails at the first request otherwise, with the resolved name nowhere in the message
    dynamo-unmapped-table-name:
      trigger: a table_names entry naming a table no definition declares
      severity: error
      reason: it does nothing, and looking correct is the whole problem
    dynamo-state-unknown:
      trigger: any run without the online gate while this store is configured
      severity: note
      message: names the deployed-versus-generated question as unanswered and points at pw migrate status
  online:
    gate: the api:cli-doctor online option, as for the relational checks
    dynamo-endpoint-unreachable:
      trigger: the configured endpoint refusing a bounded call
      severity: error
      evidence: the endpoint with credentials redacted, and the driver sentinel
    dynamo-table-missing:
      trigger: a registered table the account does not have
      severity: error
      remedy: pw migrate up in development, or the deployment tooling that owns it in production
    dynamo-key-schema-drift:
      trigger: a deployed key schema differing from the generated definition
      severity: error
      reason: it is the one check deployment tooling cannot make, per requirement:dynamodb-migration
      not_checked: billing, capacity, TTL, and every other decision:dynamodb-operational-configuration value, because a correct deployment differs there on purpose
    dynamo-untracked-table:
      trigger: a table carrying the configured prefix that no definition claims
      severity: note
      reason: it is usually a renamed type or a finished test run, and never something to delete on a diagnosis
rules:
  - the offline set runs by default and the online set never runs without the gate, so diagnosing a deployment stays a read of files
  - an online check reports its own failure as a finding rather than ending the run, and a refused connection suppresses the checks behind it as limits
  - every database interaction here is read-only; no check applies, rolls back, or creates a migration
  - the throwaway database of a snapshot comparison is local and temporary, per the api:cli-migrate snapshot action
  - policy:migration-safety confirmation rules are untouched, because doctor changes nothing
```
