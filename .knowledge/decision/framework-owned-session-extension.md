---
id: decision:framework-owned-session-extension
type: decision
title: The Framework Installs the Session
---
pw installs the session middleware and popcornweb/plugin/auth drives it, so an application that imports no authentication still has typed per-browser storage.

```yaml
status: accepted
state: implemented
problem:
  was: the extension occupying SlotSession was named auth.session and lived in plugin/auth
  effect: concept:session-storage-boundary claimed session storage does not depend on login, and the claim was false in practice
  cost: an application wanting only a locale cookie had to import authentication
ordering_already_said_it:
  fact: pw declares SlotSession at 10 and SlotAuthentication at 20, and documents that a later slot may depend on state an earlier one prepared
  reading: the framework's own ordering already separated storage from identity; only the code disagreed
the_knot:
  need_forward: the session needs a lifetime, which decision:session-lifetime-owned-by-auth places under [auth]
  need_backward: authentication needs the manager, to rotate at login and destroy at logout
  constraint: pw must not import plugin/auth
resolution:
  config: the binding structs move to popcornweb/sessionconfig, a leaf that imports nothing of the framework, and pw re-exports each as a true alias
  why_an_alias_works: the configuration registry is keyed by reflect.Type, and a true alias is the same type, so pw.SessionConfig and sessionconfig.SessionConfig resolve to one entry
  hazard: a defined type would be a different reflect.Type and the lookup would silently miss, so the alias must stay an alias
  lifetime_binding: sessionconfig declares the lifetime struct and plugin/auth binds it at the auth.session prefix, so linking authentication is what makes the keys exist
  absent: with no authentication linked the binding is unregistered and the value reads as its zero, which means bounded by the browser alone
  manager: pw.SessionManager returns what SlotSession built, which plugin/auth resolves at SlotAuthentication
  authentication: plugin/auth derives data:request-authentication in its own middleware from its own slot; the session package no longer takes a hook for it
generator_constraint:
  fact: the configbind generator resolves the type of a RegisterConfig call within the calling package
  consequence: a qualified sessionconfig.X in the call fails; a local alias declared in the calling package resolves
  effect: plugin/auth declares a local alias of the lifetime type beside its binding call
rules:
  - definition and binding stay separate: importing the leaf registers a schema, and only RegisterConfig makes keys live
  - package initialization order now guarantees a definition precedes its binding, which replaces the lexical file-name ordering pw depended on
  - plugin/auth registers its own slot through pw.RegisterSessionStore, exactly as an application does, so it is privileged only as the caller of Rotate and Destroy
consequences:
  - session.enabled alone installs the middleware
  - the extension is named session rather than auth.session
  - everything plugin/auth still does at startup was authentication setup already: the ceremony store, the owned tables, the protection patterns
```
