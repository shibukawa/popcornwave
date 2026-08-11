# contrib/oidc

`oidc` is an OpenID Connect relying-party client over `contrib/oauth`. It
validates discovery metadata, retrieves and bounds a JWKS cache, performs
Authorization Code + S256 PKCE and RFC 8628 Device Authorization, and verifies
RS256 ID Tokens including issuer, audience, `iat`, `exp`, and `azp`. Browser
callbacks additionally require nonce correlation.

`Provider` and `Client` may be used concurrently. The JWKS cache is protected
internally; the application-owned `authstate.Store` must provide the same
concurrency and atomic single-use guarantees.

```go
provider, err := oidc.Discover(ctx, "https://issuer.example", oidc.DiscoverOptions{})
store, _ := memory.NewStore[oauth.Transaction](memory.Options{})
client, err := oidc.NewClient(provider, oidc.Config{
	ClientID: "client", ClientSecret: "secret",
	RedirectURI: "https://app.example/callback",
}, oidc.Options{OAuth: oauth.Options{StateStore: store}})
url, key, err := client.BeginAuthorization(ctx, oidc.BeginOptions{Scopes: []string{"profile"}})
parsed, _ := urlpkg.Parse(url)
state := parsed.Query().Get("state")
// code is the authorization code received by the redirect handler.
tokens, err := client.HandleCallback(ctx, key, oidc.Callback{State: state, Code: code})
```

Device Flow uses the discovered `device_authorization_endpoint`:

```go
device, err := oidc.NewDeviceClient(provider, oidc.DeviceConfig{
	ClientID: "device-client",
}, oidc.DeviceOptions{})
authorization, err := device.Begin(ctx, oidc.DeviceBeginOptions{Scopes: []string{"profile"}})
// Display authorization.VerificationURI and authorization.UserCode.
tokens, idToken, err := device.Poll(ctx, authorization)
```

The example uses `urlpkg` as an alias for Go's `net/url` package.

Discovery, JWKS, and UserInfo responses are bounded, requests default to a
30-second timeout, redirects are rejected, and endpoint URLs must be HTTPS
unless loopback development is explicitly enabled. `DiscoverOptions.EndpointValidator`
can add host/IP trust policy, including resolved-IP restrictions. It receives
inspection copies of the issuer and every discovered endpoint; URL mutations
are ignored. The cache
permits one refresh for an unknown key ID, serializes concurrent refreshes, and
retains a last-valid set only through its configured stale limit. Provider
implementation, public clients outside Device Flow, dynamic
registration, implicit/hybrid flow, JWE, and `private_key_jwt` are intentionally
excluded.

JWKS freshness is bounded by the configured cache and stale TTLs. Response
`Cache-Control: max-age` can shorten freshness; `no-cache` forces immediate
revalidation and `no-store` prevents stale retention.

UserInfo responses must be bounded JSON objects containing a non-empty `sub`
claim; duplicate members and missing or non-string subjects are rejected.
Use `UserInfoWithSubject` when binding UserInfo to the subject from a verified
ID Token; it rejects a mismatched `sub` claim.
Access tokens containing control or whitespace bytes are rejected before they
are copied into the `Authorization` header.

The OIDC callback accepts only the `Bearer` token type (case-insensitive),
because UserInfo requests use the Bearer authorization scheme.

`HandleCallback` validates the nonce transaction binding before token exchange,
then binds the ID Token nonce to the atomically consumed OAuth transaction.
Code that verifies a token outside that flow must retain its own
correlation value and call `VerifyIDTokenWithNonce`; `VerifyIDToken` alone only
checks that a nonce claim is present.

The typed Device Flow completion path accepts an absent nonce because RFC 8628
has no browser transaction to bind. Signature, issuer, audience, `azp`, time,
and subject validation remain mandatory; other verification entry points do
not receive this exception.

The repository runs protocol-shaped fixtures for independent providers,
including JWKS rotation and UserInfo subject binding. This does not claim a
live provider or OpenID Foundation Conformance Suite result; the current scope
and external-test checklist are documented in
[`docs/contrib-auth-compatibility.md`](../../docs/contrib-auth-compatibility.md)
and [`docs/contrib-auth-external-conformance.md`](../../docs/contrib-auth-external-conformance.md).
