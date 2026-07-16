# internal/authn

This internal package contains only protocol-neutral bounded primitives shared
by the authentication contrib packages: cryptographic random values, strict
Base64url, constant-time secret comparison, PKCE S256, duplicate-aware JSON,
endpoint validation, redirect rejection, and bounded response reads.

Secret comparison is bounded by `MaxEncodedSecretBytes`, and bounded readers
reject nil readers before any I/O. Protocol packages should still apply their
own tighter limits at public request boundaries.

JWT algorithm/key selection and WebAuthn COSE/ES256 verification deliberately
remain in their protocol packages. A caller must not use this package to infer
an algorithm from untrusted input.
