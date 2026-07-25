---
id: api:session-manager
type: api
title: Session Manager API
---
The session manager owns cookie tokens and coordinates api:session-store without exposing backend details to handlers.

```yaml
surface:
  - ReadSession[T](context.Context) returns an immutable safe session view and presence
  - Manager[T].Create(http.ResponseWriter, *http.Request, T) error
  - Manager[T].Rotate(http.ResponseWriter, *http.Request, T) error
  - Manager[T].Delete(http.ResponseWriter, *http.Request) error
rules:
  - Create issues a new token and data:session-record
  - Create also generates a CSRF secret when policy:csrf-protection is enabled
  - Rotate revokes the previous record before issuing a replacement with a new CSRF secret
  - Delete revokes the store record and expires the browser cookie
  - handlers never receive the raw token, key hash, backend client, or mutable stored record
  - middleware supplies the validated request view through data:request-context-capsule
```
