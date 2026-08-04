---
id: decision:cookie-session-storage
type: decision
title: Cookie-Backed Session Storage
---
The cookie session backend is an ordinary api:session-store implementation, so a deployment moves between a cookie, an RDB, and a Redis-compatible store by configuration rather than by rewriting handlers.

```yaml
status: accepted
state: data:session-record sealed into a browser cookie
selection: data:session-runtime-config session.backend = cookie
model:
  token: unchanged; the browser still receives the opaque api:session-manager token
  record: a second cookie holding the sealed record
  binding: the record is sealed under the hash of its own token
  reason:
    - one contract for every backend keeps Create, Rotate, Delete, and Read[T] identical
    - binding the record to the token hash stops a record from being replayed with another token
    - a separate cookie is required because the manager owns the value of the token cookie
request_binding:
  problem: api:session-store is context-only, and a client-side store needs the request and the response
  solution: session.RequestBinder, an optional interface the manager calls before every store call it makes for a request
  effect: a backend store implements nothing and is unaffected
revocation:
  fact: this store cannot revoke a record it already wrote
  consequence: Delete expires the client copy, and a copy taken earlier stays valid until its sealed expiry
  accepted: yes, for deployments that want no session storage at all
  mitigation:
    - keep the absolute TTL short
    - rotate the sealing secret to invalidate every outstanding record at once
    - use a server-side store where sessions must end on demand
size:
  budget: session.DefaultMaxCookieBytes for the cookie name and encoded record together
  failure: an oversized record is refused at the write, because a browser drops one silently
  guidance: a payload that outgrows the budget is a signal to move to a server-side store
second_use:
  fact: this store is also the anonymous phase of a session.Private slot, whatever backend the deployment configured
  reason: decision:slot-declared-placement confines the revocation defect below to the interval before a login, where it costs nothing
  effect: the data:session-runtime-config keyring becomes required in deployments that never select this backend, though session.ReadOnly already required it wherever one was registered
suitability:
  fits: development, single-process deployments, and sessions whose payload is small and revocation is not required
  does_not_fit: immediate logout across devices, large payloads, and audit of live sessions
rejected_alternatives:
  - a session package that special-cases cookie storage instead of implementing the store contract, which would fork the manager
  - one cookie holding both token and record, which the manager cannot write without owning the value twice
  - an unbound record cookie, which would let one browser's record authenticate another browser's token
```
