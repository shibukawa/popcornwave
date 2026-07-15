---
id: requirement:contrib-passkey
type: requirement
title: TinyGo Passkey Relying Party
---
contrib/passkey implements bounded server-side WebAuthn registration and authentication for passkeys using requirement:contrib-cbor.

```yaml
package: contrib/passkey
scope: WebAuthn Relying Party
public_api:
  - BeginRegistration(user, options) returns creation options and ceremony state
  - FinishRegistration(ceremony state, credential response) returns credential record
  - BeginAuthentication(user or discoverable mode, options) returns request options and ceremony state
  - FinishAuthentication(ceremony state, assertion response, credential record) returns assertion result
  - JSON request and response records compatible with browser PublicKeyCredential data
credential_record:
  - credential ID
  - user handle
  - normalized public key and COSE algorithm
  - sign count
  - backup eligibility and current backup state
  - transports when supplied
required:
  - registration and authentication ceremonies
  - discoverable credentials and username-less authentication
  - exact challenge, origin, RP ID hash, client data type, UP, and configured UV verification
  - authenticator data parsing and assertion signature verification
  - ES256 COSE keys and signatures
  - excludeCredentials and allowCredentials
  - none attestation with explicit policy result
  - single-use expiring ceremony state supplied to caller for storage
  - policy:passkey-security
deferred:
  - packed, TPM, Android, Apple, and FIDO U2F attestation trust validation
  - metadata service integration and authenticator certification policy
  - EdDSA and RSA algorithms until TinyGo interoperability is proven
  - PRF, largeBlob, payment, related-origin, and legacy U2F extensions
  - browser JavaScript framework
standard: https://www.w3.org/TR/webauthn-3/
```
