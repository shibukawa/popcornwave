---
title: Token Revocation
description: Ending a bearer token before it expires, the two forms of revocation, and what happens when the revocation store cannot answer.
sidebar:
  order: 9
---

A verified access token is valid until it expires. That is the one property a
resource server cannot change on its own: your application did not mint the
token, and it cannot withdraw it at the issuer. So when a token leaks, or an
account turns out to be compromised, every copy of that credential keeps
working until `exp` — unless the application keeps its own list of tokens it
will no longer honor. Revocation is that list.

This page is about the [`jwt_only`
mode](/guides/backend/authentication/#jwt-only-api-servers). Browser sessions
have a different ending — logout destroys the session record — and need none
of this.

:::note[Before you start]
Revocation reads a relational table, so it requires `middleware.rdb`,
`auth.backend = "rdb"`, and the framework migration that creates
`popcornwave_auth_revocation`. A deployment on `admission = "authenticated"`
with `revocation.mode = "off"` needs no database at all; turning revocation on
is what creates the requirement.
:::

## Turning it on

`revocation.mode` has no default. Startup refuses a missing value, because
running without a revocation path should be a decision on the page, not an
omission:

```toml
[auth.jwt]
issuer = "https://issuer.example"
audience = ["orders-api"]
algorithms = ["RS256"]
admission = "authenticated"
identity_claim = "sub"
max_token_lifetime = "1h"
revocation.mode = "both"      # off, token, subject, or both
```

`off` is a legitimate answer, not a stub: a deployment whose tokens live for
five minutes may decide the exposure window is acceptable and skip the
database requirement entirely. Do not use revocation as a substitute for short
lifetimes — the list bounds the damage of a leak, it does not shrink the
window in which every ordinary token is trusted.

With any mode other than `off`, every admitted request is checked against the
list after signature and claim verification pass. A revoked token is refused
with the same `401` as an expired one — a caller probing the API cannot tell
which refusal it got, and that is intentional.

## The two forms

`token` revokes one credential; `subject` revokes an identity. They answer
different incidents, and `both` is the ordinary choice because neither
substitutes for the other.

Revoking a **token** is the narrow act, for a leaked credential whose owner is
otherwise fine. The entry is keyed on the token's `jti` claim, which is why
selecting this form makes `jti` mandatory — a token nobody can name is a token
nobody can revoke.

Revoking a **subject** is the broad act, for a compromised account: it refuses
every token issued to that identity *before now*. Enumerating the outstanding
`jti` values of a stolen account is exactly what nobody can do, so the entry
stores a timestamp instead, and the check compares it against each token's
`iat`. The account is not banned — once the person re-authenticates at the
issuer, the fresh token postdates the stamp and is admitted. Banning an
account is an [admission](/guides/backend/authentication/#who-gets-in)
decision, not a revocation.

Both calls name the issuer explicitly. The entry is scoped by it, and a call
that inferred the issuer from configuration would silently write to the wrong
scope if the deployment ever gained a second one.

## Revoking from your own code

There is no HTTP endpoint for any of this. An endpoint that revokes tokens
needs an authorization rule the framework has no basis to write, so the
surface is a set of Go calls, and your application decides who may reach them:

```go
// internal/admin/revoke.go
package admin

import (
	"net/http"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
)

const issuer = "https://issuer.example" // the auth.jwt.issuer this server verifies

// RevokeAccount withdraws every outstanding token of one identity.
// Route it however your operators reach admin actions; the framework
// deliberately mounts nothing here.
func RevokeAccount(w http.ResponseWriter, r *http.Request) {
	identityKey := r.PathValue("identity")
	err := auth.RevokeSubject(r.Context(), issuer, identityKey,
		"compromised account, support ticket 4211")
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

`auth.RevokeToken(ctx, issuer, tokenID, note)` is the single-token
counterpart. The note is for your future self reading the table during an
incident; the framework never shows it to a caller.

Revoking an identifier that was never presented **succeeds**. The caller
cannot know whether a stolen token was ever used here, and an error would make
the safe reflex — revoke first, investigate second — needlessly noisy.

A revocation entry needs no expiry from you. It lives for
`max_token_lifetime` past its stamp, which by construction outlives every
token it must refuse, and the store sweeps it afterwards.

## Undoing a mistake

`auth.ReinstateToken` and `auth.ReinstateSubject` delete an entry. This is
removal, not an undo with history: every unexpired token the entry was
refusing works again at the next request. For an administrative view that must
not guess, `auth.TokenRevoked` and `auth.SubjectRevoked` report the stored
stamp and presence, reading the store directly rather than any per-request
cache.

## When the store cannot answer

`revocation.on_unavailable` defaults to `refuse`, and the refusal is a `503`,
not a `401` — the credential was not judged, and retrying may succeed.
Admitting on an outage would turn the revocation table into an optimization
and make every revocation conditional on infrastructure being up.

The `admit` override exists as an incident lever, not a deployment posture:
with it set, every revoked token works again for the duration of the outage,
and the configuration advisories report it at error level so it does not
quietly outlive the incident.

By default every admitted request reads the store.
`revocation.max_propagation_delay` permits a small per-process cache instead,
and its value is the honest answer to "how fast does a revocation take
effect" — a token revoked now may be admitted for up to that long. Leave it at
zero unless the extra read shows up in your latency budget.

## Every key

`revocation.mode`, `on_unavailable`, and `max_propagation_delay` are listed
with the rest of the `[auth.jwt]` keys in the [configuration
reference](/reference/configuration/#authjwt).
