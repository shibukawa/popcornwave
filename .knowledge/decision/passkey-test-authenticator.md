---
id: decision:passkey-test-authenticator
type: decision
title: Passkey Test Authenticator Strategy
---
Passkey modes are tested by supplying the missing client side as requirement:contrib-passkey-test, never by weakening the server, because a deployable authentication bypass is a worse risk than an untestable flow.

```yaml
status: accepted
problem:
  - a passkey ceremony needs an authenticator, and CI has no browser, no hardware, and no human gesture
  - data:authentication-runtime-config offers oidc_only, oidc_passkey, and passkey_only, and two of the three cannot be exercised without one
decision: ship a client-side virtual authenticator and drive the real api:authentication-endpoints through it
rejected:
  - a configuration flag that accepts an unverified passkey response in development, because a misconfigured deployment then loses authentication entirely and every release must prove the flag is off
  - a testutil helper that mints a session directly, because it proves nothing about the ceremony it skips
  - a headless browser inside framework tests, because it adds a host toolchain and flake to a package whose browser JavaScript is deferred
  - leaving the authenticator inside contrib/passkey test files, because an application cannot import a _test.go fixture
manual_development:
  need: none
  reason: WebAuthn treats http://localhost and any RFC 6761 name under .localhost as a secure origin, so a platform authenticator works against api:cli-dev without TLS or tunneling
  limit: a shared or LAN-reachable development host needs real TLS and a real RP ID, which decision:local-tls-proxy-boundary already owns
  trap: an IP literal cannot be an RP ID, so 127.0.0.1 fails where localhost succeeds
browser_e2e:
  placement: application repositories, outside the framework
  method: the CDP WebAuthn virtual authenticator or the WebDriver virtual authenticator extension, each replacing the platform authenticator inside a real browser
  reason: it validates application browser code, which requirement:contrib-passkey defers
placement:
  package: contrib/passkey/passkeytest
  reason: it uses requirement:contrib-passkey credential and options types directly, and the import path names the boundary the way net/http/httptest does
  precedent: authstate already splits adapters into subpackages that carry their own requirement concepts
  constructor: NewAuthenticator rather than New, because the package is named for its purpose and not for what it returns, which is why httptest exposes NewServer, NewRecorder, and NewRequest and no bare New
  rejected:
    - the contrib/passkey package itself, for two reasons below
    - a separate contrib/devpasskey top level, because the pairing with the relying party disappears from the import path and nothing wants the authenticator without it; requirement:contrib-devidp is top level because it is a server rather than a part of requirement:contrib-oidc
    - contrib/devauthn, because contrib/internal/authn already holds the authn name
    - a testutil-only helper, because requirement:contrib-passkey fixtures under requirement:contrib-acceptance need the same authenticator without importing testutil
same_package_refused:
  build_guard: api:cli-build rejects a development-only package by dependency graph, and every passkey application imports contrib/passkey, so the guard could never fire on a type living there
  compatibility_matrix: policy:contrib-compatibility requires contrib/passkey to compile under TinyGo, and merging the authenticator would move key generation and fault injection into a target it never runs on
  surface: it would double the public API a TinyGo consumer must read before finding the relying party
independence:
  rule: contrib/passkey/passkeytest counts as at most one of the two independent authenticator implementations requirement:contrib-passkey requires
  reason: it is written in this repository against the same reading of the specification, so a shared misreading would pass on both sides
  second_source: published W3C and FIDO test vectors, which requirement:contrib-passkey already carries for the ES256 none case
sequencing: the authenticator landed before oidc_passkey and passkey_only passed startup validation, so both modes arrived with ceremony tests rather than after them
```
