---
id: rule:multi-statement-sql-execution
type: rule
title: Multi-statement SQL Execution
---
A SQL script must be split into individual statements before execution, because running a whole script in one Exec is a driver-specific convenience.

```yaml
evidence:
  host_default: the mattn backend of decision:sqlite-backend-selection executes every statement in one Exec
  tinygo_and_forced: the cgosqlite backend selected by decision:force-tinygo-logic executes the first statement and ignores the rest
  discovery: a data:migration-snapshot replay created the schema but silently skipped its INSERT statements under force_tinygo_logic
splitter:
  location: one shared internal package used by every replay path
  toolchain: no build tag; the same splitter runs on host and TinyGo
  rules:
    - a semicolon inside a single-quoted string, double-quoted or backquoted identifier, or bracketed identifier does not end a statement
    - a doubled quote is an escape, not a terminator
    - line and block comments are carried through without ending a statement
    - a CREATE TRIGGER statement ends only at a semicolon whose preceding word is END
    - a trailing statement without a semicolon is still executed
applies_to:
  - api:test-run snapshot replay
  - api:migration-runner snapshot replay
non_goals:
  - a general SQL parser
  - dialect-specific statement validation
verification: splitter unit tests plus a snapshot replay run under both toolchain selections
```
