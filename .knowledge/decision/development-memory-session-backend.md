---
id: decision:development-memory-session-backend
type: decision
title: Development Volatile Memory Session Store
---

Provide a built-in process-local implementation of `api:session-store` behind the public `dev-volatile` mode of `decision:development-session-modes`. Do not expose `memory` as general deployment vocabulary.

The memory store holds both anonymous and authenticated records. A `Private` value is server-side from its first write in this mode; anonymous-to-authenticated promotion is a store-local transition. Merely registering memory as the authenticated backend is insufficient because `decision:slot-declared-placement` would otherwise put anonymous `Private` values in the sealed cookie.

Keep the opaque session token cookie and the masked CSRF transport cookie. Do not emit the sealed session-record cookie when all declared slots are memory-placed. Require a keyring only when a declared slot still uses signed or sealed cookie placement, such as `ReadOnly`, or when `decision:cookie-session-storage` is selected explicitly.

The store is concurrency-safe, copies payloads across its boundary, enforces absolute and idle expiry, has no cross-process replication, and loses every record on process restart. A stale browser token becomes a store miss and is cleared or replaced on the next session access. Logout and state loss after restart are accepted development behavior.

`data:session-runtime-config` resolves `dev-volatile` to this store and rejects the mode outside development. Restart persistence is the separate `dev-persist` intent mode.

## Consequences

The default development loop tolerates record-schema and codec changes without retaining incompatible encrypted state. It also removes cookie encryption and record payload bandwidth from ordinary development requests.

Authentication, carts, and preferences disappear on restart. Multi-process development does not share sessions. These tradeoffs are visible in generated configuration and startup diagnostics.

The store follows `api:session-backend-plugin`, `api:session-manager`, `api:session-registry`, `data:session-record`, `flow:session-lifecycle`, and `policy:session-security`.
