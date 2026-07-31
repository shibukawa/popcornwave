// Package passkeyonlye2e holds the end-to-end test of the passkey_only mode. It
// carries no source of its own.
//
// It is a package of its own for the reason plugin/auth/passkeye2e gives:
// framework configuration is parsed once per process, so one test binary can
// build exactly one deployment, and passkey_only is a different one from
// oidc_passkey.
package passkeyonlye2e
