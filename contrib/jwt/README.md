# contrib/jwt

`jwt` implements a bounded signed JWT subset for Go and TinyGo. Verification
requires an explicit algorithm allowlist and a resolver that returns a key
bound to the same algorithm. `alg=none`, unsupported critical headers,
duplicate JSON members, non-canonical Base64url, ambiguous JWKS matches, and
oversized input are rejected.

Required verification algorithms are HS256 and RS256. Signing is provided for
HS256 through `HMACSigner`; callers may provide a custom `Signer` only for the
same HS256 algorithm. ES256, EdDSA, and JWE signing or verification are not
supported.
