# contrib/oidc

`oidc` is an OpenID Connect relying-party client over `contrib/oauth`. It
validates discovery metadata, retrieves and bounds a JWKS cache, performs
Authorization Code + S256 PKCE, and verifies RS256 ID Tokens including issuer,
audience, `iat`, `exp`, `azp`, and nonce.

`Provider` and `Client` may be used concurrently. The JWKS cache is protected
internally; the application-owned `authstate.Store` must provide the same
concurrency and atomic single-use guarantees.

```go
provider, err := oidc.Discover(ctx, "https://issuer.example", oidc.DiscoverOptions{})
store, _ := authstate.NewMemoryStore[oauth.Transaction](authstate.Options{})
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

The example uses `urlpkg` as an alias for Go's `net/url` package.

Discovery, JWKS, and UserInfo responses are bounded, requests default to a
30-second timeout, redirects are rejected, and endpoint URLs must be HTTPS
unless loopback development is explicitly enabled. `DiscoverOptions.EndpointValidator`
can add host/IP trust policy, including resolved-IP restrictions. The cache
permits one refresh for an unknown key ID, serializes concurrent refreshes, and
retains a last-valid set only through its configured stale limit. Provider
implementation, dynamic registration, implicit/hybrid flow, JWE, and
`private_key_jwt` are intentionally excluded.

JWKS freshness is bounded by the configured cache and stale TTLs. Response
`Cache-Control: max-age` can shorten freshness; `no-cache` forces immediate
revalidation and `no-store` prevents stale retention.

UserInfo responses must be bounded JSON objects containing a non-empty `sub`
claim; duplicate members and missing or non-string subjects are rejected.
Use `UserInfoWithSubject` when binding UserInfo to the subject from a verified
ID Token; it rejects a mismatched `sub` claim.

`HandleCallback` validates the nonce transaction binding before token exchange,
then binds the ID Token nonce to the atomically consumed OAuth transaction.
Code that verifies a token outside that flow must retain its own
correlation value and call `VerifyIDTokenWithNonce`; `VerifyIDToken` alone only
checks that a nonce claim is present.
