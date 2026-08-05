---
id: policy:token-revocation
type: policy
title: Bearer Token Revocation Policy
---
A verified access token is valid until it expires, which is the one property a resource server cannot change on its own; revocation gives the deployment a server-side list that refuses a token before its expiry.

```yaml
problem: a stolen token stays valid for its whole lifetime, and this application neither issued it nor can withdraw it at the issuer
answer: a bounded revocation list this application consults on every admitted request
enabled: auth.jwt.revocation.mode, stated explicitly and never defaulted; a deployment with short token lifetimes may accept the exposure by writing off, and says so
forms:
  selection: auth.jwt.revocation.mode, naming off, token, subject, or both; selecting a form is what turns its requirement on, rather than a separate switch that can disagree with it
  token:
    key: the jti claim, scoped by issuer
    revokes: one token
    requires: jti, which policy:access-token-verification already demands of an at+jwt token; selecting this form extends that demand to a relaxed token type as well
    missing_jti: refused, because a token nobody can name is a token nobody can revoke
  subject:
    key: the value of auth.jwt.identity_claim, scoped by issuer
    revokes: every token for that identity issued before a stamp
    compares: the iat claim against the stored stamp, so a token minted after the revocation is admitted and the identity keeps working once it re-authenticates
    bounded_by: auth.jwt.max_token_lifetime, which the mode requires of every jwt_only configuration, because the entry has to outlive every token it must refuse and this application does not know how long the issuer mints for
    fits: a compromised account, where enumerating the outstanding jti values is exactly what nobody can do
  both: the ordinary answer; the token form ends one leaked credential and the subject form ends a compromised identity, and neither substitutes for the other
record: data:revoked-token-record
storage:
  backend: the server backends of requirement:state-storage-tiers, selected through api:session-backend-plugin, in a keyspace of its own
  reason: a deployment running this framework has already chosen and configured one of rdb, redis, or dynamo, and a revocation list is the same shape of keyed record with an expiry
  not_a_session: the entries are not data:session-record values and carry no token, so concept:session-storage-boundary is not crossed; only the backend contract is shared
  refused_backend: cookie, which has no server storage and no revocation, per decision:cookie-session-storage
  why_not_authstate: requirement:contrib-auth-state Take is destructive by contract, and a revocation entry is read many times and consumed never
  owned_table: popcornwave_revoked_token for the rdb backend, under rule:framework-owned-tables
key_handling:
  stored: a hash of the issuer and the identifier, matching the key-hash discipline api:session-store already applies
  reason: a jti is a token identifier, and a leaked list should not be a list of live token names
expiry:
  token_form: the exp of the revoked token, plus the verification leeway
  subject_form: the stamp plus auth.jwt.max_token_lifetime
  rule: an entry must outlive every token it refuses, so a shorter TTL is a revocation that silently expires
  sweep: the backend's own, per api:session-store; nothing accumulates beyond the longest token lifetime
unavailable:
  default: refuse the request as an error, not as a denial, per auth.jwt.revocation.on_unavailable
  status: 503 rather than 401, because the credential was not judged and retrying it may succeed
  reason: admitting on an outage turns the store into an optimization and makes every revocation conditional on infrastructure being up
  override: on_unavailable admit, kept so an outage has an operator lever rather than none, and reported by rule:configuration-advisories at error level rather than as a note
  override_meaning: with it set, revocation is advisory and every revoked token works again for the duration of the outage; it is a temporary act during an incident, not a deployment posture
caching:
  default: none; a cached admit is a revocation that has not taken effect yet
  bounded: auth.jwt.revocation.max_propagation_delay, off by default, which permits a per-process cache no longer than the delay it names
  honesty: the configured delay is the answer to how fast a revocation takes effect, and there is no other
  bounded_in_size_too: the delay bounds how long an entry is trusted, and a cap on entries bounds how many are held; an unauthenticated caller chooses the keys, so the second bound is not an optimization
  eviction: an entry past the delay is dropped rather than kept and re-judged on every read
  cap: 4096 entries, swept of what the delay retired and discarded whole when that is not enough
  why_discarding_is_enough: an entry is only an optimization, it is valid for the delay at most, and losing one costs a single indexed read, so the bookkeeping an eviction order would need buys nothing
surface: api:bearer-authentication publishes the revoke and reinstate calls; no HTTP endpoint is mounted, because an endpoint that revokes tokens needs an authorization rule this framework has no basis to write
rules:
  - consult the list only after policy:access-token-verification and policy:bearer-admission both pass
  - a revoked token is refused with the revoked category of policy:access-token-verification, indistinguishable to the client from an expired one
  - revoking an unknown identifier succeeds, because the caller cannot know whether a token was ever presented
  - revoking twice is idempotent and does not extend an entry past its own expiry
  - the list is never enumerated or scanned at request time
  - audit every revocation with the actor, the reason, and the hashed key, never the jti itself
non_goals:
  - propagating a revocation to the issuer or to any other resource server
  - RFC 7009 token revocation, which is an authorization server endpoint
  - an allowlist of live tokens, which is a session store with extra steps and defeats the point of a stateless credential
```
