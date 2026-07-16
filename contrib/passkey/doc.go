// Package passkey implements bounded server-side WebAuthn registration and
// authentication for ES256 passkeys.
//
// CeremonyState contains secret correlation data and must be stored without
// logging it and consumed once. SessionFlow provides this behavior over an
// authstate.Store. CredentialRecord and AuthenticationResult are returned for
// application-owned atomic persistence.
package passkey
