---
id: api:auth-state-codec
type: api
title: Authentication State Codec
---
authstate.Codec[T] provides explicit bounded serialization for durable requirement:contrib-auth-state adapters.

```yaml
package: authstate
interface:
  - Encode(value T) returns []byte or stable error
  - Decode([]byte) returns T or stable error
rules:
  - each payload begins with an explicit format version
  - unknown versions, trailing data, malformed lengths, and oversized fields fail closed
  - stores enforce encoded-size limits before persistence and before decode
  - codec errors never include payload, credentials, challenges, verifiers, nonces, or tokens
  - no implicit gob or reflection-selected encoding
providers:
  oauth: requirement:contrib-oauth provides TransactionCodec
  passkey: requirement:contrib-passkey provides CeremonyStateCodec for private fields
compatibility:
  - codecs decode every format version still present within the maximum store TTL
  - incompatible encoding changes use a new version before deployment
```
