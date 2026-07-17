# contrib/passkey

`passkey` implements a bounded WebAuthn relying party for ES256 passkeys. It
supports registration, authentication, discoverable credentials, none
attestation, user-presence and user-verification policy, backup state, and
signature-counter risk reporting.

```go
rp, err := passkey.New(passkey.Config{
	RPID:    "example.com",
	RPName:  "Example",
	Origins: []string{"https://example.com"},
})
if err != nil {
	return err
}

store, err := memory.NewStore[passkey.CeremonyState](memory.Options{})
if err != nil {
	return err
}
flow, err := passkey.NewSessionFlow(rp, store)
if err != nil {
	return err
}

creation, transaction, err := flow.BeginRegistration(ctx, passkey.User{
	ID: []byte("stable-opaque-user-handle"), Name: "user", DisplayName: "User",
}, passkey.RegistrationOptions{RequireUserVerification: true})
```

Send `creation` to `navigator.credentials.create`. Strictly decode the browser
JSON with `DecodeRegistrationCredential`, then call `FinishRegistration` with
the transaction key. `SessionFlow` atomically consumes the state before it
validates the response, so retries and replay fail closed.

`memory.Store` is process-local. Applications with multiple processes should
replace it with an `authstate.Store` implementation whose `Take` operation is
atomic across those processes.

## Security boundaries

- Challenges contain 256 random bits and expire after five minutes by default.
- Origins are matched exactly; HTTP is accepted only for explicitly enabled
  loopback development origins.
- RP IDs are validated as bounded domain-style labels before origin scope is
  accepted.
- RP ID hashes, credential IDs, user handles, flags, ES256 algorithms, COSE
  keys, and ASN.1 DER signatures are checked before acceptance.
- JSON, Base64url, CBOR, credential IDs, authenticator data, and signatures are
  bounded. Duplicate JSON and CBOR members are rejected.
- Sign counter non-increase is returned as `CounterRisk`; the application
  decides whether to reject, warn, or accept and persists the new counter and
  backup state atomically.

Packed, TPM, Android, Apple, FIDO U2F, and metadata-backed attestation trust;
RSA, EdDSA, extensions affecting security policy; and browser framework code
are intentionally unsupported.
