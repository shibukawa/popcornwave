---
title: Session storage
description: Where a login session lives, how long it lasts, and what each backend asks of a deployment.
sidebar:
  order: 4
---

A session is two decisions that look like one. Where does the record live, and
how long does it stay valid? Both are configuration, and neither reaches the
handlers: [`session.Read`](/guides/backend/sessions/) and `auth.User` return
the same thing whether the record sits in a cookie, a database row, Redis, or
DynamoDB.

What the browser holds never changes either. The cookie carries 256 random
bits and nothing else; the store is keyed by the SHA-256 hash of that value, so
a leaked backend dump cannot be replayed as a cookie. Only one backend also
puts the record itself in the browser, and it seals it first.

## Turning it on

```toml
[session]
enabled = true
backend = "rdb"
ttl = "12h"
idle_timeout = "1h"
cookie.secure = true
```

```go
// cmd/myapp/main.go — storage is opt-in, so the backend is an import too.
import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
```

One mistake is common enough to expect: configuring a backend without
importing it. The error names its own fix.

```
session.backend = "rdb" needs its plugin; add to the application:
import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
```

The exception is `cookie`, which stores nothing and therefore imports nothing.
A project can have working sessions before it has anywhere to put them.

## Options every backend reads

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | the middleware runs only when this is true |
| `backend` | `"rdb"` | `rdb`, `cookie`, `redis`, or `dynamo` |
| `ttl` | `"24h"` | absolute lifetime; a session ends here regardless of activity |
| `idle_timeout` | `"0s"` | inactivity lifetime; zero disables it |
| `renewal_interval` | `"0s"` | how often activity may extend idle expiry; zero means a tenth of `idle_timeout` |
| `cookie.name` | `"pw_session"` | |
| `cookie.path` | `"/"` | must be rooted |
| `cookie.domain` | *(empty)* | host-only when empty, which is the safer default |
| `cookie.secure` | `true` | disable only for loopback development |
| `cookie.http_only` | `true` | |
| `cookie.same_site` | `"lax"` | `strict`, `lax`, or `none`; `none` without `secure` is rejected at startup |

`ttl` is required to be positive and at most a year. `idle_timeout` may not
exceed it. Both bounds are checked before the first request, so a contradiction
stops a deployment rather than shortening a session unexpectedly.

Renewal is the part worth understanding before tuning it. An active request
extends idle expiry, but not on every request: the store is touched only after
`renewal_interval` has passed since the record was last seen, and never past
the absolute expiry. Leave it at zero and a one-hour idle timeout writes at
most one renewal per six minutes per session. Set it lower and you buy
precision with writes.

Logging in rotates the token. The previous record is revoked before the
replacement is issued, so a token captured before authentication cannot be
reused after it — session fixation closed at the point where it matters.

## The four backends

### rdb — a row per session

The default. It needs `middleware.rdb.enabled = true`, the migration that
creates its table, and the import of the engine it runs on. See
[Relational databases](/guides/storage/rdb/).

| Key | Default | Meaning |
| --- | --- | --- |
| `rdb.source` | `"middleware"` | reuse the `middleware.rdb` pool; `dedicated` is not implemented yet |
| `rdb.group` | *(empty)* | connection group holding the table; empty resolves to `middleware.rdb.write_group` |
| `rdb.table` | `"popcornwave_session"` | |

SQLite, PostgreSQL, and MySQL all serve it, each through its own package —
`sessionstore/sqlite`, `sessionstore/postgres`, `sessionstore/mysql` — and its
own dialect of the migration. `pw init` writes the import and the migration of
the engine you selected; `pw add auth` prints the import as a manual step and
writes the migration.

Startup verifies the table rather than creating it. A deployment that skipped
the migration is told which one to apply:

```
sessionstore: session table is missing: apply the migration named
init_popcornwave_session with pw migrate up
```

Rows accumulate when a session is abandoned rather than ended, so the auth
plugin sweeps expired records every ten minutes.

### cookie — no storage at all

The record is sealed with AES-256-GCM and rides in a second cookie beside the
token, bound to that token's hash. Nothing is stored on the server, so nothing
has to be migrated, swept, or reached over the network.

| Key | Default | Meaning |
| --- | --- | --- |
| `cookie_store.secret` | *(empty)* | **required**; 32+ random bytes in base64 |
| `cookie_store.name` | `"pw_session_data"` | the cookie carrying the sealed record |
| `cookie_store.previous_secrets` | `[]` | retired secrets, still accepted for reading |

Generate the secret once and keep it out of the file:

```bash
export SESSION_COOKIE_SECRET=$(openssl rand -base64 32)
```

```toml
[session.cookie_store]
secret = "${SESSION_COOKIE_SECRET}"
```

Rotating it is the reason `previous_secrets` exists: the new secret writes,
the old ones still read, and browsers holding a record keep their session until
it expires. Drop an old secret and every record written under it stops being
accepted at once.

That last lever matters more here than elsewhere, because **this backend cannot
revoke a single session**. Logout expires the client's copy, but a copy taken
beforehand stays valid until its sealed expiry passes; there is no server
record to delete. A short `ttl` narrows the window, and a secret rotation ends
every outstanding session together. If you need to end *one* session on
demand, this is the wrong backend.

The other limit is size. A browser silently drops a cookie past about 4 KB, so
the store refuses to write one instead: `session.ErrCookieTooLarge`, at the
write, rather than a session that mysteriously never starts.

### redis — server-side expiry

Redis and Valkey both serve it, over `GET`, `SET`, `SET XX`, and `DEL` only.
Each record is written with a TTL taken from its own deadline, so an abandoned
session disappears on its own and no sweep is scheduled.

| Key | Default | Meaning |
| --- | --- | --- |
| `redis.dsn` | *(empty)* | **required**; `redis://` or `rediss://` |
| `redis.key_prefix` | `"pw:session:"` | the key space this store owns |
| `redis.connect_timeout` | `"5s"` | startup ping and per-command deadline |

```toml
[session]
backend = "redis"

[session.redis]
dsn = "redis://${REDIS_HOST}:6379/0"
```

Startup dials the server and pings it. A server that does not answer stops the
deployment instead of failing the first login. Keys stay inside the configured
prefix, and the store never scans or enumerates.

Outside loopback, reach the server through the local TLS proxy boundary rather
than a direct TinyGo TLS dial.

### dynamo — no relational database at all

A deployment already paying for DynamoDB should not have to add a relational
database for one table. This backend is that case. It borrows the client
[`database/dynamo`](/guides/storage/dynamodb/) opens — so that package has to
be enabled and imported — and opens nothing of its own.

```go
import (
	_ "github.com/shibukawa/popcornwave/database/dynamo"
	_ "github.com/shibukawa/popcornwave/sessionstore/dynamo"
)
```

| Key | Default | Meaning |
| --- | --- | --- |
| `dynamo.table` | `"popcornwave_session"` | the declared table name, mapped onto the deployed one by `middleware.dynamo` |
| `dynamo.consistent_read` | `false` | make the first read strongly consistent |

The table has one attribute for a key — the hash of the session token — and no
sort key. It is registered by the import, so `middleware.dynamo` creates it in
development along with every other table and verifies it at startup everywhere.

A read is eventually consistent by default. That is half the price, and on a
store read that happens on nearly every authenticated request, the price is the
design. The hazard it buys is narrow and real: login rotates the token, the
browser follows the redirect, and the next request may land on a replica that
has not caught up — the user looks logged out the instant they logged in. So a
read that finds nothing is retried once, consistently, and that answer is
final. A hit is never re-read, so the ordinary request still pays the cheap
read, and the second one falls only on requests that were going to be rejected
anyway. `consistent_read = true` makes the first read consistent and removes
the retry.

**Nothing here deletes an expired record.** Correctness does not depend on
deletion: a record past its deadline is reported as not found, and `Touch` is
conditioned on the record still being alive, so a renewal cannot revive one.
The bytes are another question. The store maintains a `dead_at` attribute
holding whichever expiry comes first, and removing the item is DynamoDB TTL
pointed at that attribute — enabled by whatever provisions the table, not by
anything here. Without it the table grows forever.

There is no sweep to fall back on, and that is deliberate. A sweep here is a
`Scan`: removing a million expired sessions would cost a million item reads,
where TTL removes them for nothing. The relational backend sweeps because there
the same job is one cheap `DELETE`.

A record larger than the DynamoDB item limit is refused before it is sent, with
the limit named.

## The CSRF secret

Every record carries one more field the browser never sees: a random secret the
CSRF check verifies against. It is written when the record is created and
replaced whenever the session rotates, so a token minted before a login is
refused after one. Every backend stores it, including the cookie one, which
seals it along with the rest of the record.

Two cookies go out together once `security.csrf.enabled` is on:

| Cookie | Holds | `HttpOnly` |
| --- | --- | --- |
| `pw_session` | the opaque session token | yes |
| `pw_csrf` | a token derived from the secret | **no** |

The second is deliberately readable by script, because the browser runtime reads
it when it issues a request. That is the one exception to the rule that framework
cookies are `HttpOnly`, and it buys something specific: a token read at request
time survives a rotation that a token embedded in the page would not.

Both are written at the same call site, so a browser holding a session cookie
always holds a matching token. Logging out expires both.

## Choosing

| | cookie | rdb | redis | dynamo |
| --- | --- | --- | --- | --- |
| Storage to operate | none | a table you already have | one more service | a table you already have |
| Revoke one session | no | yes | yes | yes |
| Payload size | ~4 KB, enforced | row-sized | record-sized | 400 KB, enforced |
| Who collects the abandoned | nobody; the stamp expires | a periodic sweep | the server's TTL | the table's TTL, if you enabled it |
| Import | none | `sessionstore/<engine>` | `sessionstore/redis` | `database/dynamo` and `sessionstore/dynamo` |

Read that table as one question: does this deployment need to end a session it
did not start? Cookie sessions answer no, and everything else about them
follows from that. If the answer is yes, the choice narrows to where the record
is cheaper to keep — a database you already run, or a server that expires
records for you.

Changing the answer later is a configuration edit and an import. Sessions
issued under the old backend do not migrate; users sign in again, which is why
the choice is worth making before a deployment rather than after.

Reading the record from a handler is the same call under every one of them:
[Sessions](/guides/backend/sessions/).
