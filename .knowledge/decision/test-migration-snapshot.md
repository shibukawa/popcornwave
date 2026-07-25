---
id: decision:test-migration-snapshot
type: decision
title: Test Migration Snapshot
---
api:test-run installs schema by replaying a cached SQL snapshot instead of running migrations against each isolated database.

```yaml
status: accepted
problems_solved:
  memory_database: a delegated pw child cannot reach an in-process sqlite://:memory: database, but it can emit SQL the parent replays into it
  per_test_cost: applying every migration version per test scales with migration count; replaying one snapshot does not
  path_uniformity: host Go and TinyGo tests install identical schema from the same artifact
mechanism:
  produce: api:cli-migrate snapshot applies data:migration-source to a throwaway temporary database and writes data:migration-snapshot
  transfer: the child writes the snapshot to stdout; no DSN crosses the process boundary
  cache: one snapshot per test binary, keyed by a content hash of the migration sources
  replay: each api:test-run executes the snapshot against its own isolated database
selection:
  sqlite: snapshot is the default for the first-class tier under decision:sqlite-backend-selection
  other_dialects: fall back to direct apply because a portable dump is not defined
  escape_hatch: an explicit option forces direct apply for tests that must exercise the migration path itself
alternatives_rejected:
  temp_file_per_test: simplest, but abandons in-memory databases and still pays full migration cost per test
  shared_cache_memory_dsn: file::memory:?cache=shared is shared within one process only and does not reach a child
  parent_side_temp_file_then_copy: a file copy cannot populate an already-open in-memory database
constraints:
  - the snapshot is a test fixture, never a production schema installation path
  - a snapshot is regenerated whenever the migration source hash changes
  - snapshot output is deterministic so an unchanged source yields an identical artifact
  - a fidelity gap in data:migration-snapshot is a defect, not an accepted approximation
verified:
  writable_schema_pragma: sqlite_sequence restoration works on the selected drivers; an AUTOINCREMENT counter survives replay
  round_trip: a replayed database dumps identically to a directly migrated one apart from goose timestamps
  determinism: repeated snapshots of unchanged sources are identical
  memory_database: a TinyGo-path test installs the schema into sqlite://:memory: through the child process
risks:
  driver_behavior: a future SQLite backend could reject PRAGMA writable_schema, which the round-trip test would catch
```
