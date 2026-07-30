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
rules:
  - the offline set runs by default and the online set never runs without the gate, so diagnosing a deployment stays a read of files
  - an online check reports its own failure as a finding rather than ending the run, and a refused connection suppresses the checks behind it as limits
  - every database interaction here is read-only; no check applies, rolls back, or creates a migration
  - the throwaway database of a snapshot comparison is local and temporary, per the api:cli-migrate snapshot action
  - policy:migration-safety confirmation rules are untouched, because doctor changes nothing
```
