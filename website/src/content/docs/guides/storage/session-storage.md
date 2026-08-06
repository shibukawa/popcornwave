---
title: Session storage
description: Where each declared piece of per-browser state lives, what bounds it, and what each backend asks of a deployment.
sidebar:
  order: 4
---

The [session declaration](/guides/backend/sessions/) defines what a piece of
state means. Storage configuration decides where its bytes live, how expired
records are removed, and what the backend costs to operate.

Only two of the five placements leave anything to decide. `session.Shared` and
`session.ReadOnly` are cookies by definition — a value the client reads has to
travel to it — and `session.RequestScope` lives and dies in the memory of one
request, never reaching storage at all. `session.Private` and
`session.ServerOnly` are the ones that reach a store.

What the browser holds never changes with the answer. The token cookie carries
256 random bits and nothing else, and the store is keyed by the SHA-256 hash of
that value, so a leaked backend dump cannot be replayed as a cookie.

## Turning it on

```toml
[session]
enabled = true
backend = "rdb"
retention = "720h"
keyring.secret = "${SESSION_KEYRING_SECRET}"
```

```go
// cmd/myapp/main.go — storage is opt-in, so the backend is an import too.
import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
```

One mistake is common enough to expect: configuring a backend without importing
it. The error names its own fix.

```
session.backend = "rdb" needs its plugin; add to the application:
import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
```

The exception is `cookie`, which stores nothing and therefore imports nothing. A
project can have working sessions before it has anywhere to put them, which is
why `pw init` picks it for a project with no login.

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | the middleware runs only when this is true |
| `backend` | `"rdb"` | which server store a server-placed slot uses |
| `retention` | `"720h"` | how long the store may hold one record |
| `cookie.name` | `"pw_session"` | the token cookie |
| `cookie.path` | `"/"` | must be rooted |
| `cookie.domain` | *(empty)* | host-only when empty, which is the safer default |
| `cookie.secure` | `true` | disable only for loopback development |
| `cookie.http_only` | `true` | |
| `cookie.same_site` | `"lax"` | `strict`, `lax`, or `none`; `none` without `secure` is rejected at startup |
| `keyring.secret` | *(empty)* | signs and seals anything the browser carries |
| `keyring.previous_secrets` | `[]` | retired secrets, still accepted for reading |

## Two bounds, and why both

`retention` is the only duration this section has, and it is easy to mistake for
the session lifetime. It is not.

```toml
[session]
retention = "720h"      # how long the store may hold a record

[auth]
session.ttl = "12h"     # how long a proof of identity stays good
```

They are ceilings on different things, so a record lives for whichever is
shorter. A deployment with no authentication linked has only the first, and it
still needs it: the sweep that keeps a table bounded deletes rows whose expiry
has passed, and it reads a record with no expiry as already past. A server
backend therefore refuses a non-positive `retention` at startup rather than
writing records nothing can read back.

Renewal is worth understanding before tuning it. An active request extends idle
expiry, but not on every request: the store is touched only after
`auth.session.renewal_interval` has passed since the record was last seen, and
never past the absolute expiry. Leave it at zero and a one-hour idle timeout
writes at most one renewal every six minutes per session.

## One keyring

A `session.ReadOnly` slot is signed with HMAC-SHA256 and a `session.Private`
slot is sealed with AES-256-GCM. Both derive a purpose-separated subkey from
`keyring.secret`, so a deployment configures one key rather than one per
mechanism.

It is required unless every declared slot is `session.Shared` or
`session.RequestScope` — including on
`rdb`, `redis`, `dynamo`, and `firestore`, because the anonymous phase of a private slot is a
sealed cookie whatever the backend is. `pw init` generates one into
`config.dev.toml`; every other environment reads `SESSION_KEYRING_SECRET`, and
`pw doctor --env=prod` reports a literal there as an error.

Rotating is what `previous_secrets` exists for: the first secret writes, retired
ones still read, and browsers holding a value keep it until it expires. Drop an
old secret and everything written under it stops being accepted at once — which
is also the only way a cookie-placed record can be revoked at all.

## The five backends

### rdb — a row per session

The default for a project with a login. It needs `middleware.rdb.enabled = true`,
the migration that creates its table, and the import of the engine it runs on.

| Key | Default | Meaning |
| --- | --- | --- |
| `rdb.source` | `"middleware"` | reuse the `middleware.rdb` pool; `dedicated` is not implemented yet |
| `rdb.group` | *(empty)* | connection group holding the table; empty resolves to the write group |
| `rdb.table` | `"popcornwave_session"` | |

SQLite, PostgreSQL, and MySQL each arrive through their own package —
`sessionstore/sqlite`, `sessionstore/postgres`, `sessionstore/mysql` — and their
own dialect of the migration. Startup verifies the table rather than creating
it, so a deployment that skipped the migration is told which one to apply.

Rows accumulate when a session is abandoned rather than ended, so the framework
sweeps expired records every ten minutes. That sweep is why `retention` has to
be positive here.

### cookie — no storage at all

The record is sealed with AES-256-GCM and rides in a second cookie beside the
token, bound to that token's hash. Nothing is stored on the server, so nothing
has to be migrated, swept, or reached over the network.

| Key | Default | Meaning |
| --- | --- | --- |
| `cookie_store.name` | `"pw_session_data"` | the cookie carrying the sealed record |

**This backend cannot revoke a single session.** Logout expires the client's
copy, but a copy taken beforehand stays valid until its sealed expiry; there is
no server record to delete. A short lifetime narrows the window, and a keyring
rotation ends every outstanding session together. Registering a
`session.ServerOnly` slot against it fails at startup, naming the slot: that
slot asked for revocation this backend cannot give.

The other limit is size. A browser silently drops a cookie past about 4 KB, so
the store refuses to write one instead — `session.ErrCookieTooLarge`, at the
write, rather than a session that mysteriously never starts.

### redis — server-side expiry

Redis and Valkey both serve it, over `GET`, `SET`, `SET XX`, and `DEL` only.
Each record is written with a TTL taken from its own deadline, so an abandoned
session disappears on its own.

| Key | Default | Meaning |
| --- | --- | --- |
| `redis.dsn` | *(empty)* | **required**; `redis://` or `rediss://` |
| `redis.key_prefix` | `"pw:session:"` | the key space this store owns |
| `redis.connect_timeout` | `"5s"` | startup ping and per-command deadline |

Startup dials the server and pings it, so a server that does not answer stops
the deployment instead of failing the first login. Keys stay inside the
configured prefix, and the store never scans or enumerates.

### dynamo — no relational database at all

| Key | Default | Meaning |
| --- | --- | --- |
| `dynamo.table` | `"popcornwave_session"` | declared table name |
| `dynamo.consistent_read` | `false` | strong consistency costs twice the read capacity |

It borrows the client `middleware.dynamo` already opened, so it carries no
endpoint and no credential of its own. Table TTL removes dead records, and
nothing sweeps.

### firestore — Datastore mode on Google Cloud

| Key | Default | Meaning |
| --- | --- | --- |
| `firestore.kind` | `"popcornwave_session"` | entity kind holding session records |

It borrows the client opened by `middleware.firestore` and reads each session
strongly consistently. A renewal reads the entity and rewrites it with a
version precondition, so `auth.session.renewal_interval` controls two requests
per renewal rather than one.

The stored deadline decides immediately whether a session is alive. Removing
expired bytes is separate: apply a Firestore TTL policy to `expires_at` on the
configured kind. There is no framework sweep and no migration.

## The CSRF secret

The secret the CSRF check verifies against is a registered slot like any other:
`session.Private`, declared by the framework only where `security.csrf.enabled`
is on.

That one slot serves both populations. A visitor with no login gets the secret
in the sealed cookie the anonymous phase uses, so a crawler loading a page with
a form costs a cookie and not a record. The login rotation moves the same slot
onto the configured backend and mints a fresh secret with it, which is what
stops a token minted before a sign-in from being presented after one.

Two cookies go out together:

| Cookie | Holds | `HttpOnly` |
| --- | --- | --- |
| `pw_session` | the opaque session token | yes |
| `pw_csrf` | a masked token derived from the secret | **no** |

The second is deliberately readable by script, because the browser runtime reads
it when it issues a request. That is the one exception to the rule that
framework cookies are `HttpOnly`.

## Choosing

| | `cookie` | `rdb` | `redis` | `dynamo` | `firestore` |
| --- | --- | --- | --- | --- | --- |
| Storage to operate | none | a table you already have | one more service | one more table | one more kind |
| Revoke one session | no | yes | yes | yes | yes |
| Payload size | ~3.8 KB, enforced | row-sized | record-sized | item-sized | entity-sized |
| Who collects the abandoned | nobody; the stamp expires | the framework sweep | the server's TTL | the table's TTL | a deployed TTL policy |
| Import | none | `sessionstore/<engine>` | `sessionstore/redis` | `sessionstore/dynamo` | `sessionstore/firestore` |

Read the table as one question: does this deployment need to end a session it
did not start? Answer no and `cookie` is coherent, and everything else about it
follows. Answer yes and the choice narrows to where the record is cheaper to
keep — a database you already run, or a server that expires records for you.

Changing the answer later is a configuration edit and an import. Sessions issued
under the old backend do not migrate; users sign in again, which is why the
choice is worth making before a deployment rather than after.
