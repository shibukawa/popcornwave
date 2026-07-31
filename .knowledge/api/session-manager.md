---
id: api:session-manager
type: api
title: Session Manager API
---
The session manager owns cookie tokens and coordinates api:session-store without exposing backend details to handlers.

```yaml
package: github.com/shibukawa/popcornwave/session
surface:
  - session.NewManager[T](Store[T], Options[T]) constructs a manager
  - session.Read[T](context.Context) returns an immutable safe session view and presence
  - Manager[T].Create(http.ResponseWriter, *http.Request, T) error
  - Manager[T].Rotate(http.ResponseWriter, *http.Request, T) error
  - Manager[T].Delete(http.ResponseWriter, *http.Request) error
  - Manager[T].Middleware(UnavailableHandler) installs flow:session-lifecycle
  - session.NewJar[T] is the separate api:cookie-jar for application cookies, not for login state
token:
  browser_value: 256 random bits in base64url
  store_key: SHA-256 hash of the browser value
rules:
  - Create issues a new token and data:session-record
  - Rotate revokes the previous record before issuing a replacement
  - Delete revokes the store record and expires the browser cookie
  - handlers never receive the raw token, key hash, backend client, or mutable stored record
  - middleware supplies the validated request view through data:request-context-capsule
  - Options.Subject derives the data:request-authentication subject from the typed payload
  - Options rejects an insecure SameSite none cookie, an idle timeout beyond the absolute TTL, and a non-rooted cookie path
  - a token that does not match the issued syntax never reaches the store
  - the manager binds the request and response into every store call, which a client-side store reads through api:session-store RequestBinder
  - the manager is unaware of the selected backend, so decision:cookie-session-storage changes no call site
deferred:
  - policy:csrf-protection secret generation and rotation
```
