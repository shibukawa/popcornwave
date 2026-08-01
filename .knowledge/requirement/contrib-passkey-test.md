---
id: requirement:contrib-passkey-test
type: requirement
title: Virtual Passkey Authenticator
---
contrib/passkey/passkeytest is a test-only software authenticator that answers requirement:contrib-passkey ceremonies with browser-shaped credential JSON, so a passkey flow runs without a browser, hardware, or human gesture.

```yaml
package: contrib/passkey/passkeytest
scope: test and CI client-side authenticator only
runtime: host Go only, exempt from the policy:contrib-compatibility TinyGo matrix per decision:host-tools-target-runtime
placement:
  form: a subpackage of requirement:contrib-passkey, whose credential and options types it uses directly
  separation: it is a distinct package rather than more of contrib/passkey, per decision:passkey-test-authenticator
counterpart: requirement:contrib-devidp, which plays the same role for requirement:contrib-oidc
public_api:
  - NewAuthenticator(options ...Option) returns an Authenticator holding no credential yet, because a real device mints a key per registration
  - Authenticator.Create(creationOptions) returns the RegistrationCredential a browser would post
  - Authenticator.Get(requestOptions) returns the AuthenticationCredential a browser would post
  - Authenticator.Credentials() returns stored credential ID, RP ID, user handle, and sign count
  - Authenticator.Origin() reports the origin it claims
  - WithOrigin, WithAlgorithm, WithSeed, WithDiscoverable, WithTransports, WithAAGUID, WithUserVerification, and WithBackup(eligible, state)
  - WithFault(fault) and Authenticator.SetFault(fault), which scope a fault to a run or to one call
input_contract:
  source: the exact options object requirement:contrib-passkey returned to its caller
  rule: challenge, RP ID, user handle, and credential descriptors are read from that object only
  reason: a helper handed the ceremony state directly would keep passing while the wire format is broken
required:
  - clientDataJSON carries the ceremony type, the received challenge, and the configured origin
  - authenticatorData carries rpIdHash, flags, sign count, and attested credential data on registration
  - COSE ES256 keys encoded through requirement:contrib-cbor
  - none attestation by default
  - assertion signature over authenticatorData concatenated with the SHA-256 of clientDataJSON
  - excludeCredentials refuses registration and allowCredentials selects a stored credential
  - discoverable credentials return the user handle and non-discoverable ones omit it
  - sign count increments per assertion unless configured to freeze or decrease
  - independent instances hold independent keys, so one test can model several devices and several users
  - a caller-supplied seed makes key material and credential IDs reproducible for a failing run
  - the seeded key is derived from a scalar rather than through ecdsa.GenerateKey, which mixes in global randomness and would defeat the seed
  - one instance can hold credentials for several RP IDs and origins, so scope isolation is provable
faults:
  - wrong origin, wrong RP ID hash, and stale or altered challenge
  - cleared UP, cleared UV, and invalid BE and BS combination
  - decreasing and frozen sign count
  - unsupported COSE algorithm and corrupted signature
  - mismatched or absent user handle
  - oversized JSON, Base64url, CBOR, credential ID, and extension payloads
  - reason: policy:passkey-security negative vectors must come from the same code path as the valid ones
rules:
  - the authenticator performs no relying-party check and grants no session
  - api:cli-build fails when an application under build imports contrib/passkey/passkeytest, as policy:devidp-safety already requires for its counterpart, because a shipped signing key mints assertions the relying party accepts
  - api:cli-init never scaffolds the import
  - it supplies the client side only and never substitutes for requirement:contrib-passkey verification
deferred:
  - packed, TPM, Android, Apple, and FIDO U2F attestation statements, matching the requirement:contrib-passkey deferral
  - EdDSA and RSA algorithms until requirement:contrib-passkey accepts them
  - PRF, largeBlob, and other extensions
  - CTAP transport emulation and authenticator PIN
  - browser automation, which decision:passkey-test-authenticator places outside the framework
consumers:
  - api:testutil-passkey
  - requirement:contrib-passkey fixtures under requirement:contrib-acceptance
standard: https://www.w3.org/TR/webauthn-3/
```
