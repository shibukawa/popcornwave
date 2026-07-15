---
id: policy:passkey-security
type: policy
title: Passkey Security Policy
---
Passkey ceremonies fail closed on origin, RP scope, challenge, credential ownership, flags, algorithms, and signatures while treating counters and backup state as explicit application policy inputs.

```yaml
ceremony:
  - challenges come from crypto/rand, carry at least 128 bits, expire, and are consumed once
  - registration requires clientData type webauthn.create
  - authentication requires clientData type webauthn.get
  - origins match an explicit HTTPS allowlist except configured loopback development
  - RP ID and rpIdHash match configured scope exactly under WebAuthn rules
credential:
  - require type public-key and consistent id and rawId
  - bind credential ID and user handle to the stored account
  - reject unsupported COSE key types and algorithms before signature verification
  - require UP and require UV when caller policy requests it
  - reject invalid BE and BS flag combinations
state:
  - backup eligibility is immutable after registration
  - latest backup state is returned for atomic persistence
  - sign counter decrease produces a risk signal; caller policy decides reject, warn, or accept
privacy:
  - errors do not reveal account or credential existence
  - logs exclude challenges, clientDataJSON, authenticator data, signatures, credential IDs, and user handles
  - support multiple credentials and an application-owned recovery policy
input:
  - bound JSON, Base64url, CBOR, credential ID, authenticator data, and extension sizes
  - reject duplicate security-relevant JSON and CBOR members
  - ignore no extension whose output affects configured security policy
```
