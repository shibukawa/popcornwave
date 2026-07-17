# contrib/authstate

`authstate` provides the application-owned storage boundary for browser
authentication correlation. `Store.Take` removes a value atomically before a
successful return, so replaying a state key cannot re-enter a ceremony.

The package contains only the shared `Store[T]` and `Codec[T]` contracts and
stable errors. Implementations live in subpackages:

- `contrib/authstate/memory` for bounded process-local storage
- `contrib/authstate/redis` for Redis and Valkey
- `contrib/authstate/sqlite` for single-node SQLite storage

Multi-process deployments must use a store whose `Take` operation is atomic
across processes. Durable adapters use explicit `Codec[T]` implementations;
`oauth.TransactionCodec` and `passkey.CeremonyStateCodec` preserve the private
correlation records used by those packages. Values containing challenge,
state, nonce, or verifier material must never be logged.
