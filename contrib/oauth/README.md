# contrib/oauth

`oauth` is a bounded OAuth 2.0 Authorization Code client with S256 PKCE. It
stores state and the verifier in an application-supplied atomic
`authstate.Store` and consumes the transaction before exchanging a code.

```go
store, _ := authstate.NewMemoryStore[oauth.Transaction](authstate.Options{})
client, err := oauth.NewClient(oauth.Config{
	AuthorizationEndpoint: "https://issuer.example/authorize",
	TokenEndpoint:         "https://issuer.example/token",
	ClientID: "client", ClientSecret: "secret",
	RedirectURI: "https://app.example/callback",
}, oauth.Options{StateStore: store})
url, key, err := client.BeginAuthorization(ctx, oauth.BeginOptions{Scopes: []string{"profile"}})
parsed, _ := urlpkg.Parse(url)
state := parsed.Query().Get("state")
// code is the authorization code received by the redirect handler.
tokens, err := client.HandleCallback(ctx, key, oauth.Callback{State: state, Code: code})
```

The example uses `urlpkg` as an alias for Go's `net/url` package.

HTTPS is required except explicitly enabled loopback HTTP. Token responses are
bounded and duplicate JSON members are rejected. Redirects are disabled for
token exchange, requests default to a 30-second timeout, and errors never
include authorization codes or token values. Client IDs and secrets are
bounded to 4 KiB at configuration time. `Options.TransactionValidator` can
apply protocol-specific checks after state/PKCE validation and before the
token request. Validator failures are normalized to `ErrState`; validators
must not log or retain transaction secrets.
`Config.EndpointValidator` can additionally enforce caller-specific host/IP
trust policy for both configured endpoints; its failures become
`ErrInvalidConfig`.
The package intentionally excludes authorization-server behavior, implicit and
resource-owner-password grants, device flow, DPoP, and automatic refresh.
