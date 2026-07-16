# contrib/authstate

`authstate` provides the application-owned storage boundary for browser
authentication correlation. `Store.Take` removes a value atomically before a
successful return, so replaying a state key cannot re-enter a ceremony.

`MemoryStore` is bounded, expiry-aware, race-safe, and process-local. Multi-
process deployments must provide a `Store` backed by an atomic database or
cache operation. Nil contexts and nil store receivers return a stable
configuration error instead of panicking. Values are immutable by convention
and must never be logged when they contain challenge, state, nonce, or verifier
material.
