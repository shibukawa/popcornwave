---
title: Sessions
description: Reading the current session from a handler, and what a missing one means.
sidebar:
  order: 2
---

By the time a handler runs, the session question is already answered. The
middleware read the cookie, hashed it, fetched the record, checked both
deadlines, and either installed a validated session or did not. A handler asks
what that came to:

```go
func mypage(w http.ResponseWriter, r *http.Request) {
	view, ok := session.Read[auth.SessionData](r.Context())
	if !ok {
		// An anonymous request is a normal state, not an error.
		return
	}
	_ = view.Data       // the typed payload
	_ = view.ExpiresAt  // the authoritative deadline
	_ = view.Method     // how it was authenticated, such as "oidc"
}
```

`Read` is generic over the payload type, and the type parameter is a check
rather than a cast: a record holding some other payload reports `false` the
same way no record at all does.

| Field | Meaning |
| --- | --- |
| `Data` | the typed payload |
| `CreatedAt` | when the session began |
| `AuthenticatedAt` | when it last passed a login, which token rotation resets |
| `LastSeenAt` | the last activity the store recorded |
| `ExpiresAt` | the absolute deadline |
| `IdleExpiresAt` | the inactivity deadline; zero when none is configured |
| `Method` | how it was authenticated, such as `"oidc"` |
| `Version` | the payload schema version, which invalidates older records |

Every timestamp here is the server's; nothing the browser sends gets to claim
one. So a "your session ends at" line renders from `ExpiresAt`, and the
deadline that moves as the user works is `IdleExpiresAt`.

With `plugin/auth`, [`auth.User`](/guides/backend/authentication/#reading-the-user)
is the friendlier accessor over the same record. Either way a handler never
sees the token, the key hash, or the backend client. It never sees which
backend it was, either — which is what lets one deployment keep sessions in a
table and the next in DynamoDB without a handler edit between them. Where the
record actually lives is [Session storage](/guides/storage/session-storage/).

The view carries no CSRF secret. The record holds one, and it is deliberately
not here: nothing in a handler needs it, and keeping it out of reach is cheaper
than remembering not to log it. [Security](/guides/architecture/security/) covers
what the check does with it.

## When there is no session

`ok` is false for a request that never had a session, and equally for one whose
cookie is missing, malformed, or expired. Those all continue as an explicitly
unauthenticated request, with the stale cookie cleared, because a browser
holding a cookie the server will not honour should stop sending it.

A backend that cannot be reached is a different answer. The middleware responds
`503` rather than quietly downgrading the request to anonymous: "the database
is down" and "you are not signed in" must not look the same to an application
deciding what to show. A page that renders a sign-in prompt during a storage
outage has told the user something false, and invited them to fix it by logging
in again.

So `Read` never has to decide whether a request is allowed — by the time it
returns `false`, that question has already been answered somewhere it can be
answered correctly. Guarding a route is
[Authentication](/guides/backend/authentication/). `Read` is for the handlers
that render differently depending on who is there.
