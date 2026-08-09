---
id: rule:batch-engine-capability
type: rule
title: Batch Capability Per Engine
---
What batching is worth on each dialect of rule:rdb-dsn-resolution, how a caller reaches it, and what it costs there, because this is the one SQL capability that differs per engine in kind rather than only in syntax.

```yaml
status: written up 2026-08-09 as guides/storage/batching, both locales
purpose: the fact table requirement:native-pgx-escape-hatch wrote its
  documentation from; the framework implements none of this itself
postgres:
  transport: pgx SendBatch, one pipelined extended-protocol exchange
  reached_by: api:native-connection-access, because pw.DB reports absence on a
    requirement:pgx-native-execution connection
  exec: yes
  query: yes, rows delivered per queued statement
  cost: one round trip
  failing_index: exact; pgx names the queued statement that failed
  transaction: joins an open data:transaction-scope and keeps the one round
    trip, per decision:escape-hatch-transaction-scope
  atomicity_without_a_scope: >
    everything between the first message and the Sync is one implicit
    transaction, so a failure rolls the whole batch back and later statements
    never run
  prepared_statements: >
    already in use, and the batch does not disturb them. The default exec mode
    is cache_statement with a per-connection LRU, and every mode but the simple
    protocol sends the batch over the extended protocol, so a queued statement
    is prepared and reused exactly as an unbatched one is
  caveats:
    parse_before_execute: >
      the extended protocol may parse every queued statement before any of them
      executes, so DDL followed by use of what it created fails inside a batch
      where it would succeed inside an explicit transaction. This is the
      sharpest difference from ordinary execution and the guide states it
    non_transactional_statements: an implicit transaction is still a
      transaction block, so VACUUM, CREATE DATABASE, and CREATE INDEX
      CONCURRENTLY are rejected in any batch of more than one statement
    isolation: cannot be set for the implicit transaction; a batch needing
      REPEATABLE READ or SERIALIZABLE runs inside api:transaction-runner, which
      is where an isolation level belongs anyway
  verdict: the only engine where a batch is a batch, and the reason
    requirement:native-pgx-escape-hatch exists
sqlite:
  transport: sqlbatch sequential execution, one statement at a time, inside one
    transaction
  reached_by: nothing new is needed; api:transaction-runner opens that same
    transaction
  exec: yes
  query: yes
  cost: no round trips to save; what it saves is the fsync each autocommit
    statement would otherwise pay, measured at 200 inserts in ~50ms against ~1ms
  failing_index: exact, because each statement is executed separately
  prepared_statements: unchanged from ordinary execution; the transport calls
    ExecContext and QueryContext on the leased connection, so database/sql
    handles statements there exactly as it does outside a batch
  verdict: >
    the batch adds nothing over pw.Transaction, because it is pw.Transaction
    with a queue in front. The win belongs to the transaction, so the guidance
    is to open one and write ordinary statements inside it
mysql:
  transport: sqlbatch, statements joined into one multi-statement command
  reached_by: pw.DB, which is a real *sql.DB on this engine, so a caller uses
    sqlbatch directly with no framework surface involved
  exec: yes, with per-statement RowsAffected and LastInsertID
  query: refused; a queued read is served only by the sequential fallback
  cost: one round trip, or three when the transport adds its own transaction
  failing_index: usually unknown, reported as -1; the server returns one error
    for the whole command and the driver returns no result to count against
  transaction: cannot join an open scope at all, because the transport enters
    through sql.Conn.Raw and sql.Tx has none
  atomicity: the transport wraps the joined statement in a transaction, which
    DDL breaks by committing implicitly
  size_ceiling: the rendered statement is checked against maxAllowedPacket, and
    an oversized batch arrives as the same skip error as the causes below
  prepared_statements:
    exclusive_with_the_batch: >
      a multi-statement command cannot be prepared, which is the reason
      interpolateParams appears in required_dsn: the driver renders arguments
      into the SQL text client-side instead of binding them server-side.
      Choosing the batch on mysql is choosing to give those statements up as
      prepared ones, by the same mechanism security_posture warns about
    the_alternative: without interpolateParams a statement carrying arguments is
      prepared and then executed, two round trips each; the batch trades N of
      those for one interpolated command
  required_dsn:
    multiStatements: true, always; the capability is negotiated at handshake so
      no batch can turn it on
    interpolateParams: true whenever a queued statement carries arguments
    detection: >
      neither flag is readable from the connection, so a refusal is produced by
      failing and translating. The driver spends one skip error on three causes
      — interpolateParams off, an argument it cannot inline, and a statement
      over max_allowed_packet — so the message names all three
  framework_does_not_default_them:
    precedent_considered: the engine already normalizes parseTime=true, per
      rule:rdb-dsn-resolution, so normalizing more would be consistent in form
    why_not: >
      multiStatements widens the blast radius of any injection that reaches the
      SQL text, on every connection, for every deployment, whether or not it
      ever batches. That is a security posture change the framework must not
      make on an operator's behalf to enable an optimization they did not ask
      for
    instead: the operator sets both flags knowingly, and the guide presents the
      batch as a trade rather than a default
  security_posture: with multiStatements on, an injection reaching the SQL text
    can append a second statement; with interpolateParams on, the driver
    escapes values itself instead of sending them out of band
  verdict: reachable today with no framework change, and worth it only for a
    write-heavy path whose operator accepts both flags
snapshot_exception:
  what: reads inside one batch do not necessarily share a snapshot
  why: postgres takes a fresh one per statement at READ COMMITTED, while sqlite
    and mysql InnoDB share one across the containing transaction
  consequence: nothing at any layer can reconcile this, so a batch promises
    order and atomicity and not a consistent view
rules:
  - the framework prepares nothing explicitly, because the sqlbind executor
    seam of requirement:pgx-native-execution carries exec and query and no
    Prepare; statement reuse is the driver's, automatic on postgres and absent
    by construction inside a mysql batch
  - capability is a static property of the dialect and the connection kind,
    never a runtime probe, except the mysql DSN flags which cannot be read
  - a batch is never described only as faster; each engine entry states what it
    costs, and two of the three cost something
  - no framework code depends on this table; it governs documentation and the
    postgres accessor, nothing else
verification:
  - done: the postgres exec, transaction-joining, and rollback claims, through
    api:native-connection-access against postgres 17
  - outstanding: the sqlite fsync numbers are quoted from
    system:tinygodriver rather than re-measured here, and the guide attributes
    them to the driver for that reason
  - out_of_scope: the mysql claims stay system:tinygodriver's, which owns and
    tests that transport; this catalog records them rather than re-verifying
```
