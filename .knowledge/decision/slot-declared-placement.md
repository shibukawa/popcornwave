---
id: decision:slot-declared-placement
type: decision
title: Slot-Declared Placement and Anonymous Promotion
---
Each registered slot declares where it lives rather than inheriting one deployment-wide backend, and a private slot rides a sealed cookie until the login that promotes it to the server, so a language preference stays a readable cookie, a credential stays revocable, an anonymous visitor costs the server nothing, and a fact that must be fresh is rebuilt every request instead of stored.

```yaml
status: accepted
state: designed, per api:session-registry
supersedes:
  was: one session.backend key placing every private slot in the deployment
  problem:
    - a language preference and a credential have different answers and were given one
    - an anonymous visitor who writes anything gets a server row, so bots and one-off visits accumulate rows that will never be logged in
    - a slot that must be revocable had no way to say so, and a deployment running backend cookie silently made it unrevocable
values:
  count: five; four place bytes somewhere, and session.RequestScope is the answer for a value that must not be placed at all
  session.Shared:
    placement: cookie, necessarily; a value the client writes cannot live on the server
    protection: policy:cookie-value-protection plain
  session.ReadOnly:
    placement: cookie, necessarily; a value the client reads has to travel to it
    protection: policy:cookie-value-protection signed
  session.Private:
    anonymous: a sealed cookie, always, whatever the configured backend
    authenticated: the data:session-runtime-config backend, from the login onward
    protection: sealed either way, so the client reads nothing at any point
    default: yes; session.ServerOnly is the one that needs a stated reason
  session.ServerOnly:
    placement: the configured server backend, always, including while anonymous
    refuses: backend cookie, at startup, naming the slot
    argument: revocation, not confidentiality; sealing already hides a value from the client, but decision:cookie-session-storage cannot take one back
    cost: an anonymous write creates a server record, which is exactly what this value is asking for
  session.RequestScope:
    placement: process memory of one request; no cookie, no record, no backend, no keyring
    written_by: middleware or a handler, after deriving the value from an authoritative source; later handlers in the same request read it, and the next request starts empty
    argument: freshness; a value rebuilt from the source of record every request cannot be stale, so a revocation there is seen at the next request rather than chased through cache invalidation
    cost: the rebuild, once per request that needs it
    refuses: session.ExpiresAfter, session.OutlivesSession, and session.ResetOnRotate at registration, because the lifetime is the request and nothing else
    survives: Rotate and Destroy within its request, because the session stored nothing of it to take back
distinction:
  private_vs_server_only: whether an anonymous write reaches the server, and therefore whether the value is revocable before a login
  request_scope_vs_the_rest: whether any bytes exist after the response; the other four persist and differ in where, this one declines persistence
  everything_else: identical; both sealed tiers are opaque, both are destroyed together, and neither is readable by the client
promotion:
  what: a session.Private slot moving from its sealed cookie to the configured backend
  when: the login rotation, which policy:session-security already requires for fixation resistance
  mechanism: none added; api:session-manager Rotate already revokes the old record and creates a replacement carrying the same slot values
  consequence: promotion and rotation are the same event by construction, so no separate hook, no second write path, and no window where a session is half-promoted
  resolution: placement is resolved when a record is created rather than when the process starts
  one_way: yes; a promoted value never returns to a cookie, because the session holding it is destroyed at logout rather than demoted
  no_op: a deployment running backend cookie promotes nothing, because the destination is where the value already is
why_it_is_safe_where_it_is_weak:
  claim: the only real defect of a cookie-placed record is that decision:cookie-session-storage cannot revoke it
  observation: revocation matters after a login and not before; no deployment needs to end an anonymous session across devices on demand
  conclusion: promotion confines the cookie backend's defect to exactly the interval where it costs nothing, which is why this is the default rather than an opt-in
costs:
  anonymous_ceiling:
    fact: an anonymous private slot is bounded by session.DefaultMaxCookieBytes, 3800 bytes for the cookie name and encoded value together
    overflow: the write is refused; it does not spill to the server
    reason: spilling would put both placements live for one slot mid-session, which is the complexity promotion was meant to avoid
    guidance: state that can grow without bound while anonymous is declared session.ServerOnly and pays for its row
    visibility: the refusal names the slot and its budget, so it is a development-time failure rather than a production surprise
  bandwidth:
    fact: a sealed record cookie rides every request the cookie path covers, including static assets
    trade: server storage is saved and per-request bytes are spent, on anonymous traffic, which is usually the largest share
    opt_out: session.ServerOnly, declared per slot, because the deployment-wide answer was the thing this decision removed
  secret:
    correction: this is not a new requirement; session.ReadOnly already needed the keyring, because policy:cookie-value-protection signs with HMAC-SHA256 and session.Keyring serves the signed and sealed modes from one secret
    rule: the keyring is required unless every registered slot is session.Shared, which protects nothing, or session.RequestScope, which never leaves the process
    added_by_this_decision: session.Private under a server backend, which previously needed no keyring and now needs one for its anonymous phase
    static: the framework cannot know whether a private slot will be written before a login, so the requirement is stated at registration rather than discovered at the first anonymous write
    development: generated rather than authored, per data:session-runtime-config development_generation, so the cost lands on deployment rather than on getting started
  replay:
    fact: a client keeping a sealed anonymous record can present it at a later login and resurrect that state
    bounded_by: the expiry stamp sealed into the value, which policy:cookie-value-protection already enforces over the cookie attributes
    accepted: yes; the state is one the same browser wrote, and promotion is the only path from client-held to server-held
records:
  fact: a session may hold one cookie-placed record and one server-placed record at the same time, when a session.ServerOnly slot is written while anonymous
  bound: two placements at most, plus one cookie per client-tier slot
  atomicity: each placement is replaced atomically; a write spanning both is not one transaction
  failure: a partial write fails the request rather than committing one placement, so the two never silently disagree
rejected_alternatives:
  - keeping one deployment-wide backend, which forced a credential and a language preference into one answer
  - a separate session.Promoted value beside session.Private, which split one tier into two over a question with one good answer; a private slot has to put an anonymous write somewhere, and the cookie is the only destination that costs nothing
  - making promotion a deployment opt-in, which would leave the default creating server rows for visitors who never log in
  - spilling an oversized anonymous slot to the server, which keeps both placements live for one slot and reintroduces the migration promotion was meant to make free
  - promoting on a schedule or a size threshold rather than at login, which invents a moment that no security event marks
  - copying server state back into a cookie at logout, which would hand the client state the server had decided to end
```
