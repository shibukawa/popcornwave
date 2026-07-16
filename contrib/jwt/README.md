# contrib/jwt

`jwt` implements a bounded signed JWT subset for Go and TinyGo. Verification
requires an explicit algorithm allowlist and a resolver that returns a key
bound to the same algorithm. `alg=none`, unsupported critical headers,
duplicate JSON members, non-canonical Base64url, ambiguous JWKS matches, and
oversized input are rejected.

Required verification algorithms are HS256 and RS256. Signing is provided for
HS256 through `HMACSigner`; callers can supply another `Signer` only when its
algorithm exactly matches the protected header. ES256, EdDSA, and JWE are not
supported.
