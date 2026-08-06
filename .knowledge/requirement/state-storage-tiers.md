---
id: requirement:state-storage-tiers
type: requirement
title: Browser State Storage Tiers
---
An application states what a client may do with a piece of state, and where it lives, at the line that declares its type, so the deployment is left with the one choice it is actually qualified to make.

```yaml
audience: actor:application-developer
declaration: api:session-registry, where the tier is the placement argument of pw.RegisterSessionStore[T]
decision: decision:slot-declared-placement
axes:
  trust: what the client may do with the value
  placement: where the bytes live
  combined: the two are declared as one value, because the valid combinations are few and the invalid ones are nonsense
  lifetime: what ends the value, stated beside the tier and independent of it, per decision:slot-lifetime-axis
tiers:
  shared:
    placement: session.Shared
    client: reads and writes
    mechanism: policy:cookie-value-protection plain
    server_storage: impossible by definition; a value the client writes cannot live on the server
    handling: request input, validated like a query parameter
    fits: display density, dismissed notices, last-used tab
  read_only:
    placement: session.ReadOnly
    client: reads, cannot change
    mechanism: policy:cookie-value-protection signed
    fits: locale, tenant label, a flag the client may see but not choose
    caution: the payload stays readable, so it carries no secret
  private:
    placement: session.Private
    client: neither reads nor changes
    mechanism: a sealed cookie while anonymous, and the backend the deployment selected from the login onward
    backends: cookie, rdb, redis, dynamo, firestore
    ceiling: the anonymous phase is bounded by the browser cookie budget, and an oversized write is refused rather than spilled
    default: yes
    fits: authorization facts, the plugin/auth slot, a cart an anonymous visitor starts and a logged-in user keeps
  server_only:
    placement: session.ServerOnly
    client: neither reads nor changes
    mechanism: the configured server backend, always, including while anonymous; backend cookie is refused at startup
    argument: revocation, not confidentiality
    cost: an anonymous write creates a server record
    fits: a stored secret the client must never hold even sealed, such as a refresh token taken at login, and anything that can grow past the cookie budget while anonymous
    not: validity that must be re-checked per request, which is request_scope; a preference following the account across browsers, which belongs in the application database because a session names one browser and dies at logout
  request_scope:
    placement: session.RequestScope
    client: nothing; the value never travels
    mechanism: process memory of the request that wrote it; no cookie, no record, no keyring
    lifetime: one request, fixed; every lifetime option is refused at registration
    handling: derived from an authoritative source by middleware or a handler, read by later handlers in the same request, absent in the next
    argument: freshness over cost; rebuilding every request is what makes staleness impossible
    fits: the scope set a bearer token resolves to against the auth database, per-request authorization facts, a tenant plan read from the row of record
choosing:
  is_it_rebuilt_from_an_authoritative_source_every_request: request_scope
  can_the_front_end_change_it: shared
  can_the_front_end_read_it: read_only
  must_it_be_revocable_before_a_login_or_can_it_outgrow_a_cookie: server_only
  otherwise: private, whose anonymous phase costs the server nothing and whose backend the deployment picks
backend_selection:
  scope: which server backend the private and server_only tiers use; never whether a slot is server-placed, and never the request_scope tier, which no backend touches
  key: data:session-runtime-config backend
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
    cost: one write per change and one renewal write per interval, plus an expiry sweep
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
  dynamo:
    import: _ "popcornwave/sessionstore/dynamo"
    storage: requirement:dynamodb-session-store
    revocation: immediate
    expiry: table TTL, per decision:dynamodb-session-expiry
  firestore:
    import: _ "popcornwave/sessionstore/firestore"
    storage: requirement:firestore-session-store
    revocation: immediate
    expiry: a field TTL policy, per decision:firestore-expiry-policy
    reads: strongly consistent with no option to weigh, unlike dynamo
    cost: a renewal is a read and a write rather than one conditional write, per decision:firestore-conditional-writes
    mode: the database must have been created in Datastore mode, per decision:firestore-datastore-mode-only
one_session:
  fact: every registered slot shares one token, whatever tier each one carries
  records: a session may hold a cookie-placed record and a server-placed record at once, which happens while it is anonymous and holds a session.ServerOnly slot
  lifetime: one, supplied by decision:session-lifetime-owned-by-auth
  destruction: a logout destroys every slot that did not declare session.OutlivesSession, per flow:session-lifecycle; a request_scope value survives it within its request, because the session stored nothing of it
  survival: state that must outlive a logout is not a slot, and uses api:cookie-jar directly
invariants:
  - one Codec, one registration call, and one typed read across every tier and backend
  - moving a value between tiers changes the placement argument and nothing a handler calls
  - moving between server backends changes data:session-runtime-config and nothing a handler calls
  - the server decides every authoritative lifetime, whichever tier holds the value
  - a handler cannot tell where its slot is stored, or whether a private one has been promoted
  - policy:session-security governs the token whichever tiers a session carries
  - policy:cookie-value-protection governs anything the browser carries
acceptance:
  - a shared value survives a client edit as ordinary input, and every other tier rejects it
  - a request_scope value set in one request reads absent in the next, with no cookie written and no store touched
  - a stored record key matching a request_scope slot never populates it
  - a registry holding only shared and request_scope slots starts without a keyring
  - the same handler compiles and passes against the cookie, rdb, redis, dynamo, and firestore server backends
  - switching session.backend needs no migration of application code, only of stored records
  - a cookie-placed write beyond the browser budget is refused, naming the slot, instead of writing one the browser drops
  - a server-placed slot answers a logout by revoking the record, and a cookie-placed one states that it cannot
  - a session.ServerOnly slot registered against backend cookie fails startup naming the slot
  - selecting a backend whose plugin is not imported fails startup instead of linking every backend into every binary
  - a tier that carries a secret is never satisfied by the shared or read_only tiers
  - an anonymous browser writes a shared and a private slot, logs in, and finds both values unchanged
  - the private slot occupies no server storage before that login and no cookie after it
  - a session.ServerOnly slot written while anonymous occupies server storage immediately
  - a private slot that outgrows the cookie budget while anonymous fails the write rather than creating a server record
non_goals:
  - a client-writable value backed by server storage
  - automatic migration of live records between backends
  - spilling an oversized cookie-placed slot to the server
  - demoting a server-placed value back into a cookie, at logout or at any other moment
  - reading one tier through another tier's API
  - a slot outliving the session while living in the session record, which is not a policy declined but a thing that cannot happen
  - session affinity, replication topology, or failover between server backends
  - caching, which stores derived data rather than one client's state and shares a backend product at most
```
