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
  - configured RP ID is a bounded domain-style label set before scope matching
  - RP ID and rpIdHash match configured scope exactly under WebAuthn rules
enrollment_binding:
  rule: a registration ceremony completes for the principal that began it, or it fails closed
  stored: the ceremony state carries the enrolling account, and the data:account-bootstrap-credential ticket identity when one began it
  compared: finish requires exact equality against the principal it resolves for itself, before the credential is built
  reason: the finish handler resolves the enroller again from the live request, so without the comparison the ceremony adopts whoever is authenticated at that moment rather than whoever started it
  mechanism:
    carried: RegistrationOptions.Binding, opaque to the library and stored with the ceremony, bounded at 256 bytes
    required: SessionFlow FinishRegistration takes the binding as an argument rather than an option, so a caller cannot leave a ceremony unbound by forgetting it
    compared: constant time, before the credential is built; a mismatch consumes the state and fails like a bad challenge, and reports nothing about which half was wrong
    value: plugin/auth labels the two enrolment paths apart, so a ticket-begun ceremony cannot be finished by an ordinary session of the same account or the reverse
    not_a_secret: it is one side of an equality the server checks, so an account identifier is enough and it does not have to be unguessable
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
