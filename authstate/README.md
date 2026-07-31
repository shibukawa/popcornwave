# authstate

`authstate` provides the application-owned storage boundary for browser
authentication correlation. `Store.Take` removes a value atomically before a
successful return, so replaying a state key cannot re-enter a ceremony.

The package contains the shared `Store[T]` and `Codec[T]` contracts, the stable
errors, and `SQLStore[T]`, which runs on whatever engine a registered dialect
describes. The engines and the other backends live in subpackages, and the
import of one is what puts it in a binary:

- `authstate/sqlite`, `authstate/postgres`, and `authstate/mysql` for the SQL
  engines
- `authstate/redis` for Redis and Valkey
- `authstate/memory` for bounded process-local storage

```go
import _ "github.com/shibukawa/popcornwave/authstate/postgres"

store, err := authstate.NewSQLStore[oauth.Transaction](db, oauth.TransactionCodec{},
	authstate.SQLOptions{Dialect: "postgres", Namespace: "auth-oidc"})
```

The dialect is the name the DSN scheme already resolved to, so a deployment
names its engine once. `Take` is one statement where an engine has `RETURNING`
and a locking transaction where it does not, which is why the engines are
whole implementations of four operations rather than a table of SQL fragments.
`TestEngineContract` runs one suite against all three.

Multi-process deployments must use a store whose `Take` operation is atomic
across processes. Durable adapters use explicit `Codec[T]` implementations;
`oauth.TransactionCodec` and `passkey.CeremonyStateCodec` preserve the private
correlation records used by those packages. Values containing challenge,
state, nonce, or verifier material must never be logged.
