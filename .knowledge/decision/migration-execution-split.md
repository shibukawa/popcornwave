---
id: decision:migration-execution-split
type: decision
title: Migration Execution Split
---
A host Go build applies migrations in process, and a TinyGo build delegates the same request to the pw command as a child process.

```yaml
status: accepted
reason: system:goose is host-only, so a TinyGo application or test binary cannot link the engine
selection:
  tag_switch: decision:force-tinygo-logic
  in_process: "!tinygo && !force_tinygo_logic"
  delegated: "tinygo || force_tinygo_logic"
  api_parity: both selections implement the same api:migration-runner surface
scope:
  applies_to:
    - application startup apply
    - api:test-run from host Go tests and from TinyGo tests
  does_not_apply_to:
    - api:cli-migrate, which links the engine directly per decision:goose-migration-engine
    - api:cli-dev, which invokes pw migrate in the host toolchain
in_process:
  - open or reuse the pool from decision:config-driven-database
  - construct a goose Provider over data:migration-source
  - apply and report the resulting version
delegated:
  child: pw migrate <action>
  transport:
    dsn: PW_MIGRATE_DSN in the child environment, never a process argument
    embedded_sources: an fs.FS is staged to a temporary directory because a child cannot share it
    rationale: policy:migration-safety forbids credentials in argv
  requirements:
    - pw is resolvable on PATH from the running environment, which data:project-config already assumes for the Devbox shell
    - data:migration-source is readable by the child process
    - the DSN names a database the child process can reach
  constraint:
    in_memory_database: an in-process sqlite://:memory: database is invisible to the child, so a delegated apply requires a file-backed DSN
    test_resolution: decision:test-migration-snapshot removes this constraint for api:test-run by transferring SQL instead of a DSN
  failure:
    - a missing pw binary is a clear actionable error, not a silent skip
    - an unreachable delegated DSN is reported as a configuration error naming the constraint
    - a snapshot that fails to replay is reported with the failing statement
    - the child exit status and stderr are propagated unchanged
host_ci:
  - force_tinygo_logic exercises the delegated path under host Go tests
  - both paths assert identical applied version and error mapping
```
