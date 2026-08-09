---
id: requirement:native-pgx-escape-hatch
type: requirement
title: Direct pgx Reach On A Native Connection
---
A requirement:pgx-native-execution connection must offer the direct-to-pgx escape hatch requirement:contrib-postgresql already promises, because the native migration removed the only route to it and left that promise stale.

```yaml
status: implemented 2026-08-09
priority: should
surface: api:native-connection-access
transaction: decision:escape-hatch-transaction-scope
batch_capability: rule:batch-engine-capability
problem:
  the_claim: >
    requirement:contrib-postgresql records sql.Conn.Raw as the escape hatch
    keeping Batch, CopyFrom, and LISTEN/NOTIFY reachable without widening the
    framework surface, and requirement:pgx-native-execution repeats it when it
    excludes those three from that surface
  the_reality: >
    Raw needs a *sql.DB, and pw.DB reports absence on a native connection. A
    request has two doors, pw.DB and the sqlbind executor, and neither reaches
    pgx. The claim now holds only for the migration and seeding handles of the
    requirement:pgx-native-execution audit
  the_cost: >
    the engine with the best batch transport is the one a request cannot batch
    on. The same is true of CopyFrom and LISTEN/NOTIFY, which were already
    supposed to be reachable and are the reason the escape hatch was recorded
  not_a_regression_anyone_reported: >
    the audit moved every framework consumer off *sql.DB deliberately and
    completely. What it did not carry across was the caller-facing hatch, which
    no framework code uses and therefore no test missed
scope:
  is: one accessor on the postgres engine package that hands the pgx connection
    to a callback, plus the documentation of what each engine offers
  is_not:
    - a framework batch API; nothing named Batch enters pw
    - a portable surface across engines. sqlite and mysql already reach their
      own batch path through pw.DB, per rule:batch-engine-capability
    - a wrapper, a queue type, a result type, or an error type of our own
  why_so_small: >
    the batch, copy, and notify transports are all built and verified upstream.
    What was missing was reach, so reach is the whole requirement. A queue API
    over three engines would have been a second implementation of something
    only one engine benefits from
must:
  - hand the caller the same pgx connection type on both compilers, so callers
    compile under TinyGo and host Go alike
  - run the callback inside the active data:transaction-scope when one is open,
    per decision:escape-hatch-transaction-scope
  - resolve the connection from the effective group, exactly as
    api:database-selection resolves one statement
  - refuse a connection that is not pgx with a named error rather than a panic,
    so a sqlite or mysql group says so
  - keep pgx out of every pw signature; the accessor lives in the engine package
    an application already blank-imports
documentation:
  owner: the database guide, written from rule:batch-engine-capability
  postgres: batching through this accessor, with the parse-before-execute and
    snapshot caveats stated rather than implied
  sqlite: >
    api:transaction-runner is the whole feature. The sequential batch path is
    one statement at a time inside one transaction, which is what
    pw.Transaction already opens, so the fsync win needs no new API and the
    guide says exactly that
  mysql: >
    sqlbatch reached directly through pw.DB, with the two required DSN flags,
    the exec-only limit, the unknown failing index, and the loss of server-side
    prepare all stated. Presented as a trade to take deliberately, not a
    recommended default
  observability: >
    every entry shows the span and the log the caller writes by hand, because
    none of this work reaches api:instrumented-sql-executor. pw.StartSpanKind
    and pw.Logger are already public and already correlate, so the guide
    teaches a pattern instead of announcing an API; the shape is the
    observability recipe of api:native-connection-access
  honesty: a guide that describes a batch only as faster is wrong on every
    engine here; each entry names what it costs, and the logging section says
    plainly that the automatic query log stops at the accessor
acceptance:
  - a handler reaches pgx SendBatch, CopyFrom, and LISTEN/NOTIFY on a postgres
    group, on both compilers
  - work done inside the callback while api:transaction-runner holds a
    transaction is rolled back with that transaction
  - the accessor on a sqlite or mysql group returns a named error
  - the requirement:contrib-postgresql escape_hatch line describes something
    that exists again
  - the guide covers all three engines and states a cost for each
  - the guide shows a span and a log written by hand around the work, and the
    records land under that span with no wiring
verified:
  unit: >
    a nil callback, a context carrying no connection, and a sqlite group whose
    error names the dialect, all without a server
  integration: >
    against postgres 17 on a pool bounded to one connection: a queued batch on
    the pool, a second call that only succeeds because the first released, and
    a batch inside the transaction runner whose rows are gone after the
    rollback. That last case is the whole of
    decision:escape-hatch-transaction-scope, so it is asserted rather than
    reasoned about
  suites_pass: database and pwruntime under -race
  documentation: >
    guides/storage/batching in both locales, with both code examples compiled
    verbatim before publication, and the interoperability appendix corrected in
    both locales where it implied a second pool was the only option
later:
  automatic_tracing: >
    installing a pgx tracer on the pool config would observe batches and copies
    without wrapping anything, which is the only way to close the gap rather
    than document it. It waits on system:tinygodriver aliasing the batch tracer
    interface, and is a separate requirement when it lands
```
