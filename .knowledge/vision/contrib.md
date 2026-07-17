---
id: vision:contrib
type: vision
title: TinyGo Contrib Vision
---
Petitweb contrib provides focused TinyGo-compatible server packages where standard or common Go implementations are unavailable, incomplete, or too reflection-heavy.

```yaml
root: contrib/
packages:
  - requirement:contrib-auth-common
  - requirement:contrib-auth-state
  - requirement:contrib-cbor
  - requirement:contrib-passkey
  - requirement:contrib-otel
  - requirement:contrib-reverse-proxy
  - requirement:contrib-jwt
  - requirement:contrib-oauth
  - requirement:contrib-oidc
  - requirement:contrib-database
  - requirement:contrib-redis-valkey
  - requirement:contrib-html-template
  - requirement:contrib-zstd
principles:
  - policy:contrib-compatibility
  - policy:outbound-transport-security
  - decision:contrib-delivery-order
  - decision:passkey-first-authentication
acceptance: requirement:contrib-acceptance
scope_rule: implement explicit useful subsets; never claim full upstream compatibility without conformance evidence
```
