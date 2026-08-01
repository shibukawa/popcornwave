# sessionstore

A `database/sql` backed `session.RawStore`. The package owns the
`popcornwave_session` table and never inspects application tables.

The engine is not compiled in here. One package per engine describes its
dialect and registers it, and that import is what makes `session.backend =
"rdb"` resolve — against that engine and no other:

```go
import _ "github.com/shibukawa/popcornwave/sessionstore/postgres"
```

`sqlite`, `postgres`, and `mysql` are implemented. They share every statement
whose only difference is placeholder style; an engine package supplies the DDL,
the upsert, the bounded delete, and the catalog query, because those are the
places the engines genuinely disagree. `TestEngineContract` runs one suite
against all three, which is what keeps them interchangeable.

The import registers the backend and opens nothing. At startup the framework
hands it the pool of the RDB middleware, and the backend verifies its own table
before the application serves a request — a deployment that skipped the
migration is told which migration to apply.

Constructing it directly stays available. The store is not generic; the payload
type is added by the host:

```go
store, err := sessionstore.NewStore(db, sessionstore.Options{Dialect: "postgres"})
err = store.VerifySchema(context.Background())
typed := session.Typed[Data](store, session.JSONCodec[Data]{})
```

The dialect is the name the DSN scheme already resolved to, so a deployment
names its engine once.

The caller owns `db`, because a session store commonly shares the pool of the
RDB middleware.

The table is migration-owned. `MigrationSQL` returns the goose migration a
project carries, `MigrationName` is the name it carries it under, and
`VerifySchema` checks the result at startup without changing the schema. The
version prefix belongs to the project: the file takes the next free number when
it is written, so installing this later renumbers nothing. `EnsureSchema` still
creates the table for a test or a tool that has no migration directory.

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
