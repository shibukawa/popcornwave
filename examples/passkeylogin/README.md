# Passkey login

`auth.mode = "oidc_passkey"`: OpenID Connect creates the account, and a passkey
is the everyday sign-in. The identity provider is
[contrib/devidp](../../contrib/devidp/README.md), which `pw dev` starts for you
— nothing external to install and no credential to copy.

For a deployment with no provider at all there is `auth.mode = "passkey_only"`:
`pw init --auth=passkey` scaffolds it, and an administrator issues the login ID
and one-time secret that open a first enrollment.

## Why two methods

A passkey cannot create an account: there is nothing to attach the first
credential to. Something has to establish who this person is before they enroll
one, and in this mode the provider does. Afterwards the provider is needed only
for recovery, which is what `recovery.policy = "oidc"` records.

## What the framework does

Importing `plugin/auth` (through `handlers/accounts.go`) installs three
extensions:

| Slot | Responsibility |
| --- | --- |
| session | resolves the `pw_session` cookie into a validated session |
| authentication | serves `/auth/login`, `/auth/callback`, `/auth/logout`, and the four `/auth/passkey/*` endpoints |
| guard | redirects unauthenticated requests to paths in `auth.protection.include` |

The application registers two seams and nothing else:

| Seam | Question it answers |
| --- | --- |
| `auth.SetAccountResolver` | which local account this **verified OIDC identity** belongs to |
| `auth.SetAccountLookup` | which local account this **account ID** is, after a passkey assertion resolved one |

They point in opposite directions, which is why a passkey mode needs both: a
credential names an account directly, and there is no external identity to link.

## Reach it by name, not by address

`http://localhost:8080`, not `http://127.0.0.1:8080`.

A WebAuthn relying party is scoped to a **domain**, and an IP literal can never
be one. `localhost` is also a secure origin for WebAuthn, so development needs
no TLS and no tunnel — which is why this sample runs over plain HTTP.

## The browser half

The framework serves the endpoints but cannot call `navigator.credentials` for
the page, so [public/passkey.js](public/passkey.js) does. It has no
dependencies and does one interesting thing: convert between the Base64url the
endpoints speak and the ArrayBuffers the WebAuthn API wants.

## Tables

Framework tables are prefixed `popcornwave_` and come from their own migration
files, beside the application's:

| Migration | Tables | Owner |
| --- | --- | --- |
| [00001_init.sql](migrations/00001_init.sql) | `accounts`, `external_identities` | this application |
| [00002_init_popcornwave_session.sql](migrations/00002_init_popcornwave_session.sql) | `popcornwave_session` | `plugin/session/rdb` |
| [00003_init_popcornwave_auth.sql](migrations/00003_init_popcornwave_auth.sql) | `popcornwave_authstate`, `popcornwave_auth_allowlist`, `popcornwave_passkey_credential`, `popcornwave_auth_bootstrap` | `plugin/auth` |

`popcornwave_passkey_credential` holds the credentials. An application that
owns its own credential storage installs `auth.SetCredentialStore` instead, and
the framework then neither creates nor verifies that table.

## Run it

```
devbox shell
pw migrate up
pw dev
```

Open `http://localhost:8080`, sign in with OpenID Connect, then **Add a
passkey**. Sign out and use **Log in with a passkey**: no user name is asked
for, because the credential names the account.

## Tests

```
go test ./...
```

[handlers/passkey_test.go](handlers/passkey_test.go) drives the whole story
without a browser and without hardware:
[contrib/passkey/passkeytest](../../contrib/passkey/passkeytest) answers the
ceremony with the JSON a browser would post. It also proves a forged assertion
is refused — a tampered signature, a wrong origin, and a stale challenge each
fail the login.
