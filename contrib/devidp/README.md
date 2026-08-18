# contrib/devidp

`devidp` is a development-only OpenID Provider. It authenticates by letting a
developer pick a user from a TOML roster; no credential is ever checked. It
exists so `contrib/oidc` has an in-repo counterparty: discovery, JWKS, RS256 ID
Tokens, Authorization Code with mandatory S256 PKCE, Device Authorization,
UserInfo, and RP-initiated logout.

**Never link this package into an application binary.** `pw build` fails a
project that imports it.

```go
roster, err := devidp.LoadConfig("devidp.toml")
server, err := devidp.Start(ctx, "127.0.0.1:0", roster, devidp.Options{
	LoginUser: "admin", // skip the selection screen; omit for the login UI
})
defer server.Close()

credentials, err := server.RegisterClient(devidp.ClientSpec{LoopbackRedirects: true})
// server.Issuer(), credentials.ID, and credentials.Secret configure the client.

deviceCredentials, err := server.RegisterPublicDeviceClient(devidp.PublicDeviceClientSpec{})
// Device clients receive an ID only. No embedded client secret is created.
```

`Start` binds loopback only and derives the issuer from the resolved port, so
the caller never has to reserve one. `RegisterClient` generates the client id
and secret; with `LoopbackRedirects` it accepts any loopback callback following
the RFC 8252 §7.3 loopback rule, which lets a tool register a client before it
knows the application's port or callback path. Clients declared in the roster
keep exact redirect URI matching.

## Roster

```toml
[idp]
valid_scopes = ["admin"]     # beyond openid, profile, and email
token_ttl = "1h"             # default 1h, capped at 12h
code_ttl = "1m"              # default 1m, capped at 10m
# issuer = "http://127.0.0.1:18080"   # Start fills this from the listener
# signing_key = "signing.pem"         # default: an ephemeral RSA key per process

[users.admin]
display_name = "Administrator"
extra_scopes = ["admin"]
# subject = "admin"          # defaults to the table key
[users.admin.claims]
email = "admin@example.com"
role = "admin"
employee_id = 42

[clients.myapp]              # optional; a running tool registers its own
secret = "development-secret"
redirect_uris = ["http://127.0.0.1:8080/auth/callback"]

[clients.mydevice]
grants = ["device_code"]
```

Unknown keys and tables are errors. `iss`, `sub`, `aud`, `exp`, `iat`, `nbf`,
`auth_time`, `nonce`, `azp`, and `at_hash` cannot be set as claims. Roster
claims are always issued; `profile` and `email` additionally gate the standard
claims of those scopes. A granted scope must be present in the request, in the
provider scope set, and — beyond the standard scopes — in the user's
`extra_scopes`.

## Using it

`pw dev` starts the provider when `dev.idp.enabled` is set in
`popcornweb.toml`, registers an ephemeral client, and injects
`AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`, and `AUTH_OIDC_CLIENT_SECRET` into
the application process. Environment values outrank TOML, so no issuer or
credential belongs in a committed config file. An edited roster reloads in
place through `Provider.Reload`, keeping the issuer and injected credentials
valid.

In tests, `testutil.WithIdentityProvider` starts the same provider beside a
`TestRun` server and `WithLoginUser` pre-selects the subject, so a login
completes without driving a browser.

`/end_session` implements RP-initiated logout: it verifies `id_token_hint`
against its own signing key, revokes the access tokens of that subject, and
redirects to `post_logout_redirect_uri`. Relying parties need it — a logout that
only drops the application cookie leaves this provider signed in.

Unlike a real provider, the post-logout URL needs **no registration**: any local
target is accepted (a loopback address, `localhost`, or a name under
`.localhost`). Requiring registration would put friction back into the one place
this provider removes it from. Non-local targets are still refused, so it cannot
become an open redirect for anything off the machine.

Refresh tokens, client credentials grants, EntraID compatibility, public
clients outside Device Flow, dynamic registration, implicit and hybrid flows, consent, and
any form of credential storage are intentionally excluded. Authorization codes,
access tokens, and the signing key live in memory only and are destroyed by
`Close`.
