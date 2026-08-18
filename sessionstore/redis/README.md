# sessionstore/redis

A Redis and Valkey backed `session.RawStore`. The package owns a key space
under a configured prefix, never scans or enumerates keys, and implements no
Redis protocol client of its own — it uses `github.com/redis/go-redis/v9`.

Importing it is what makes `session.backend = "redis"` resolve, and what puts
the client in the binary at all:

```go
import _ "github.com/shibukawa/popcornweb/sessionstore/redis"
```

The import registers the backend and opens no connection. At startup the
backend dials `session.redis.dsn`, pings it before the application serves, and
hands the framework the close it will need at shutdown.

Constructing it directly stays available. The store is not generic; the payload
type is added by the host:

```go
client := goredis.NewClient(options)
store, err := redis.NewStore(client, redis.Options{KeyPrefix: "pw:session:"})
typed := session.Typed[Data](store, session.JSONCodec[Data]{})
```

A directly constructed client belongs to the caller, who closes it.

Expiry belongs to the server. Every record is written with a TTL taken from its
own deadline, so a session that is never revoked disappears without a sweep —
there is no `Prune` here, unlike `sessionstore/sqlite`. The stored deadline is
still authoritative on read: a record the server has not collected yet is
reported as `session.ErrExpired` rather than served.

`Touch` re-writes the record with `SET XX`, so a renewal never recreates a key
that expired between the read and the write, and never renews past the absolute
expiry. `Get` and `Delete` need no more than `GET` and `DEL`.

Record timestamps live in a fixed-width header ahead of the encoded payload, so
renewal rewrites the header and leaves the payload encoding alone. A key that is
not a canonical store key hash never reaches the server.

Server and client failures become `session.ErrUnavailable` without copying
their text, which can contain a DSN or a value, into the response path.

The commands used are `GET`, `SET` with an expiry, `SET XX`, and `DEL` — the
subset `requirement:contrib-redis-valkey` verified against Redis 8.4 and Valkey
9.1. Outside loopback, reach the server through the local TLS proxy boundary
rather than a direct TinyGo TLS dial.

Run the interoperability tests against both servers:

```bash
./scripts/test-session-redis.sh
```
