---
id: api:cookie-jar
type: api
title: Typed Cookie Jar
---
One typed browser cookie, read and written through the same API whichever protection policy:cookie-value-protection mode it carries, for state that lives outside the session.

```yaml
package: github.com/shibukawa/popcornwave/session
relation_to_the_registry:
  inside_the_session: api:session-registry is the declared surface, and a client-tier slot is a jar the framework constructs and destroys with the session
  outside_the_session: a jar constructed directly, for a cookie that must outlive a logout or belongs to no session at all
  example: the policy:session-downgrade hint, which exists precisely because the session it describes has ended
  guidance: prefer a registered slot; reach for a direct jar when the cookie has to survive api:session-manager Destroy
tier_selection: requirement:state-storage-tiers
surface:
  - session.NewJar[T](Codec[T], JarOptions) returns a jar; a nil codec uses session.JSONCodec[T]
  - Jar[T].Middleware() decodes the cookie once per request and publishes the handle
  - Jar[T].Read(context.Context) returns the request value and its presence
  - Jar[T].Value(context.Context) returns the request handle with Get, Set, and Clear
  - Jar[T].Load(*http.Request), Save(http.ResponseWriter, T), and Clear(http.ResponseWriter) work without the middleware
keyring:
  - session.NewKeyring(secrets ...[]byte) over raw secrets
  - session.ParseKeyring(secrets ...string) over the base64 form a configuration carries
  - required by the signed and sealed modes, unused by plain
options:
  mode: defaults to sealed, so a jar declared without one does not hand state to the client
  cookie: browser policy shared with api:session-manager; Name is required and has no default
  max_age: browser lifetime and, in a protected mode, the accepted lifetime
  max_bytes: cookie name and encoded value together, defaulting to session.DefaultMaxCookieBytes
  lifetime: stated per jar, because a cookie outside the session is outside decision:session-lifetime-owned-by-auth
errors:
  - ErrCookieMissing for a request without the cookie
  - ErrCookieInvalid for a value this jar did not write
  - ErrExpired for a protected value past its stamp
  - ErrCookieTooLarge for a value the browser would drop
  - ErrCodec for a payload the codec rejects
rules:
  - the typed API does not change with the mode, so a cookie is promoted without touching handlers
  - middleware clears an invalid or expired value and continues with an absent one
  - Set writes Set-Cookie immediately and therefore precedes response commitment
  - a write is visible to the rest of the request without a re-read
  - two jars never read each other's value, even over one type and one keyring
  - a jar is not destroyed by a logout, which is the reason to use one and the reason not to put session state in one
  - login state belongs to the plugin/auth slot of api:session-registry, not to a jar
context:
  storage: one context value per installed jar, keyed by the jar itself
  relation_to_capsule: a jar is application state, so policy:context-value-storage leaves it outside data:request-context-capsule
  cost: one value per jar an application installs, which is the depth an application chose
```
