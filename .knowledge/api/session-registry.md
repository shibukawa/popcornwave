---
id: api:session-registry
type: api
title: Typed Session Storage Registry
---
An application declares each piece of per-browser state once, as a Go type with a placement, and reads it back by that type, so no handler carries a jar, a store, or a string key.

```yaml
package: github.com/shibukawa/popcornwave/session, registered through pw
status: implemented
model: concept:session-storage-boundary
registration:
  call: pw.RegisterSessionStore[T](key, placement, lifetime...)
  caller: main, after every package init, exactly as api:runtime-configuration requires of RegisterConfig
  reason: the registry must be complete before the first request decodes anything, and an init-time call cannot see the configuration that places it
  key: the cookie name when the slot is cookie-placed, and the field name inside data:session-record when it is server-placed
  uniqueness: a duplicate Go type and a duplicate key are each a registration panic, not a silent replacement
  codec: session.JSONCodec[T] by default, overridable per slot
  bound: session.DefaultMaxCookieBytes for anything the browser carries; a server-placed value is bounded by its backend
placement:
  decision: decision:slot-declared-placement
  session.Shared:
    client: reads and writes
    where: a plain cookie, necessarily; a value the client writes cannot live on the server
    handling: request input, validated like a query parameter
    fits: display density, dismissed notices, last-used tab
  session.ReadOnly:
    client: reads, cannot write
    where: a signed cookie, necessarily; a value the client reads has to travel to it
    fits: locale, tenant label, a flag the client may see but not choose
    caution: the payload stays readable, so it carries no secret
  session.Private:
    client: neither reads nor writes
    where: a sealed cookie while the session is anonymous, and the data:session-runtime-config backend from the login onward
    default: yes; session.ServerOnly is the one that needs a stated reason
    ceiling: an anonymous value over the cookie budget is refused, not spilled, so state that can grow while anonymous is declared session.ServerOnly instead
    fits: authorization facts, the plugin/auth slot, a cart an anonymous visitor starts and a logged-in user keeps
  session.ServerOnly:
    client: neither reads nor writes
    where: the configured server backend, always, including while anonymous
    refuses: backend cookie, at startup, naming the slot
    reason: revocation; sealing hides a value from the client but decision:cookie-session-storage cannot take it back
    cost: an anonymous write creates a server record, which is what this value asks for
    fits: a credential, and anything that must stop being valid on demand
  selection: the value states both what the client may do and where the bytes go; the deployment is left only with which server backend
lifetime:
  decision: decision:slot-lifetime-axis
  default: stating nothing ties the slot to the session
  session.ExpiresAfter: bounds the slot to a duration whatever its placement, so a value may die before the session that carries it
  session.OutlivesSession: keeps the slot for a duration and exempts it from the destruction of the session; cookie-placed slots only
  session.BrowserMax: the longest a browser keeps a cookie, 400 days, which is what stating it indefinitely can mean
  rule: a slot may always state a shorter life; only a cookie-placed slot may outlive the session, because a record is destroyed with the session that holds it
  refusal: registration, not startup, because the placement is known where the lifetime is stated
surface:
  - session.Load[T](context.Context) returns the request value and its presence
  - session.Value[T](context.Context) returns a handle with Get, Set, and Clear
  - a Set is visible to the rest of the request without a re-read
  - an unregistered T is a programming error reported at the call, not an empty value
resolution:
  key: the Go type, so a package reads its own state without importing the package that declared the layout
  reason: this is the api:runtime-configuration Config[T] shape, and an application already knows it
  consequence: two packages wanting one slot share the type, which makes the sharing visible in the import graph
issuance:
  timing: lazy; the token and any record are created by the first Set, never by a bare read
  effect: a visitor who writes nothing receives no cookie and occupies no storage
  anonymous: a session exists before any login, so a cart or a draft survives the login that follows
  cost: an anonymous session touches the server only where a session.ServerOnly slot was written, per decision:slot-declared-placement
promotion:
  when: the login rotation, which policy:session-security already requires
  what: every session.Private slot moves from its sealed cookie to the configured server backend, keeping its value
  how: nothing beyond api:session-manager Rotate, which already revokes and recreates
  after: the slot is server-placed for the rest of the session, and the anonymous cookie is expired
  no_op: a deployment running backend cookie promotes nothing, because the destination is where the value already is
session_lifetime:
  source: decision:session-lifetime-owned-by-auth, which bounds the session every slot hangs off
  cookie_placed: a slot that stated nothing carries the session lifetime as its cookie lifetime, which is what makes it die with the session rather than at the next browser close
destruction:
  logout: destroys every slot that did not declare session.OutlivesSession, whatever each one's placement, per flow:session-lifecycle
  rotation: preserves every slot value and changes only the token and, for a private slot, the placement
  outside_the_session: state that must survive a logout uses api:cookie-jar directly, which is why policy:session-downgrade keeps its hint there
rules:
  - the typed API does not change with the placement, so changing one is a registration-line edit and nothing a handler calls
  - the typed API does not change with the backend, so decision:cookie-session-storage changes no call site
  - a handler never sees the token, the key hash, the placement, the codec, or the backend client
  - a handler cannot observe whether a private slot has been promoted yet; only its value is readable
  - a returned value is treated as immutable; a change is written through Set
  - two slots never read each other's value, even over one type-compatible layout, because the key is the registered type identity
  - a slot carrying a secret is never satisfied by session.Shared or session.ReadOnly
  - a slot that must be revocable declares session.ServerOnly rather than relying on the configured backend
  - a write refused for exceeding the cookie budget names the slot and its budget
  - the framework registers the plugin/auth slot the same way an application registers its own, as session.Private
supersedes:
  manager_payload: the single generic payload of api:session-manager, which let one application hold exactly one piece of session state
  constructed_jars: per-package session.NewJar plumbing, which api:cookie-jar keeps only for a cookie living outside the session
decisions:
  - decision:type-keyed-session-storage for the type-keyed registration
  - decision:slot-declared-placement for the placement values and the promotion
  - decision:slot-lifetime-axis for the lifetime, which is independent of both
installed_by: decision:framework-owned-session-extension
```
