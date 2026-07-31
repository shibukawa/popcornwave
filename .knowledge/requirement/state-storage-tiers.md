---
id: requirement:state-storage-tiers
type: requirement
title: Browser State Storage Tiers
---
An application picks how much a client may do with a piece of state first, and only the opaque tier then picks where the bytes live, so storage is a deployment choice rather than an application rewrite.

```yaml
audience: actor:application-developer
axes:
  trust: what the client may do with the value, which selects the tier
  placement: where the bytes live, which only the opaque tier chooses
tiers:
  client_owned:
    client: reads and writes
    mechanism: api:cookie-jar plain mode
    server_storage: impossible by definition; a value the client writes cannot live on the server
    handling: request input, validated like a query parameter
    fits: display density, dismissed notices, last-used tab
  client_visible:
    client: reads, cannot change
    mechanism: api:cookie-jar signed mode
    fits: locale, tenant label, a flag the client may see but not choose
    caution: the payload stays readable, so it carries no secret
  opaque:
    client: neither reads nor changes
    mechanism: sealed cookie, or an opaque token naming a server record
    backends: cookie, rdb, redis
    fits: login sessions, authorization facts, anything whose meaning is the server's
backend_selection:
  scope: inside the opaque tier only
  cookie:
    storage: none; the sealed record rides in a second cookie bound to its token hash
    import: none; it is the built-in backend
    revocation: none, per decision:cookie-session-storage
    size: one browser cookie budget
    processes: every process holding the secret reads it, with no shared infrastructure
    fits: development, single-process deployments, small payloads
  rdb:
    import: _ "popcornwave/sessionstore/<engine>", one package per engine
    storage: api:session-store rdb plugin over an existing database
    revocation: immediate
    size: bounded by the row, not by the browser
    processes: every process sharing the database
    cost: one write per login and one renewal write per interval, plus an expiry sweep
    fits: deployments that already run a database and must end a session on demand
  redis:
    import: _ "popcornwave/sessionstore/redis"
    storage: requirement:contrib-redis-valkey keyed records with native expiry
    revocation: immediate
    size: bounded by the record
    processes: every process sharing the endpoint
    cost: one more managed dependency
    status: implemented
    expiry: the server owns it, so no sweep runs
    fits: session volume or renewal rate a relational store should not absorb
invariants:
  - one Codec, one Options, one Store contract, and one Read across every tier and backend
  - promoting a cookie between tiers changes the declared mode and nothing a handler calls
  - moving between opaque backends changes data:session-runtime-config and nothing a handler calls
  - the server decides every authoritative lifetime, whichever tier holds the value
  - policy:session-security governs the opaque tier regardless of backend
  - policy:cookie-value-protection governs anything the browser carries
acceptance:
  - a plain cookie survives a client edit as ordinary input, and a signed or sealed one rejects it
  - the same handler compiles and passes against the cookie, rdb, and redis opaque backends
  - switching session.backend needs no migration of application code, only of stored records
  - a cookie backend refuses a record beyond the browser budget instead of writing one the browser drops
  - a server backend answers a logout by revoking the record, and a cookie backend states that it cannot
  - selecting a backend whose plugin is not imported fails startup instead of linking every backend into every binary
  - a tier that carries a secret is never satisfied by the client_visible tier
non_goals:
  - a client-writable value backed by server storage
  - automatic migration of live records between backends
  - reading one tier through another tier's API
  - session affinity, replication topology, or failover between opaque backends
  - caching, which stores derived data rather than one client's state and shares a backend product at most
```
