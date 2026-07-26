# plugin/session/rdb

A `database/sql` backed `session.Store[T]`. The package owns the
`popcornwave_session` table and never inspects application tables.

```go
store, err := rdb.NewStore[Data](db, session.JSONCodec[Data]{}, rdb.Options{})
err = store.VerifySchema(context.Background())
```

The caller owns `db`, because a session store commonly shares the pool of the
RDB middleware.

The table is migration-owned. `MigrationSQL` returns the goose migration a
project carries as `00001_init_popcornwave_session.sql`, and `VerifySchema`
checks it at startup without changing the schema. `EnsureSchema` still creates
the table for a test or a tool that has no migration directory.

Record timestamps are columns, not payload fields, so renewal updates one row
without rewriting or re-encoding the payload. `Get` treats stored expiry as
authoritative and reports `session.ErrExpired` regardless of what the browser
sent. `Touch` refuses to revive a missing or expired record and refuses a
renewal past the absolute expiry.

Backend failures are reported as `session.ErrUnavailable` without copying
driver text, which can contain a DSN or query fragment, into the response path.

Schedule `Prune` to remove records that expire without ever being revoked;
`plugin/auth` runs it periodically for the store it creates.

SQLite is the supported dialect. `EnsureSchema` emits SQLite DDL, and another
dialect needs its own schema until a provider covers it.
