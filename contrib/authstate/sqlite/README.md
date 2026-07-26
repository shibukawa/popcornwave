# contrib/authstate/sqlite

This package adapts Petitweb's portable SQLite facade to `authstate.Store[T]`.
It owns the `popcornwave_authstate` table, consumes records with one
`DELETE ... RETURNING` statement, and exposes bounded expiry pruning. `SchemaSQL`
publishes the DDL so a project can carry it in a migration; `EnsureSchema`
creates the table when it is missing and otherwise validates its layout. Protect
the database file and backups as secret application state.

```go
db, err := dbsqlite.Open("app.db")
store, err := sqlitestore.NewStore[oauth.Transaction](
	db,
	oauth.TransactionCodec{},
	sqlitestore.Options{Namespace: "oauth"},
)
err = store.EnsureSchema(context.Background())
```

The caller owns the shared `*sql.DB`. Schedule bounded `Prune` calls to remove
expired records that are never consumed.
