---
id: data:migration-snapshot
type: data
title: Migration Snapshot
---
A migration snapshot is a deterministic SQL script that reproduces a fully migrated SQLite database, including engine version state, when replayed into an empty database.

```yaml
producer: api:cli-migrate snapshot
consumer: api:test-run replay under decision:test-migration-snapshot
dialect: sqlite only
generation:
  - apply data:migration-source to a throwaway temporary file database
  - read object definitions from sqlite_master excluding internal sqlite_ objects
  - read every user table row in primary key order
structure:
  - PRAGMA foreign_keys=OFF
  - BEGIN TRANSACTION
  - CREATE TABLE statements with their rows as INSERT statements
  - sqlite_sequence restoration guarded by PRAGMA writable_schema ON and OFF
  - CREATE VIEW, CREATE TRIGGER, and CREATE INDEX statements after all data
  - COMMIT
fidelity_rules:
  - carry goose_db_version rows verbatim so the replayed database reports the same applied versions
  - carry seed and reference data written by migrations, not schema alone
  - render every value through the SQLite quote function, which emits x'hex' for BLOB, NULL, doubled single quotes for TEXT, and a round-trippable REAL
  - emit indexes, triggers, and views after data so constraints and triggers do not fire during load
  - exclude sqlite_master rows whose sql is null, which are implicit indexes
execution: rule:multi-statement-sql-execution
determinism:
  - stable object ordering by type then name
  - stable row ordering
  - no timestamps or paths from the producing environment beyond values already stored in rows
caching:
  key: content hash of data:migration-source
  scope: one artifact per test binary run
invalidation: any change to the migration source hash
non_goals:
  - a production backup or restore format
  - a cross-dialect portable dump
  - a human-maintained schema file committed to version control
```
