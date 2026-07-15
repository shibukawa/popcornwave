---
id: vision:contrib
type: vision
title: TinyGo Contrib Vision
---
Petitweb contrib provides focused TinyGo-compatible server packages where standard or common Go implementations are unavailable, incomplete, or too reflection-heavy.

```yaml
root: contrib/
packages:
  - requirement:contrib-httpmux
  - requirement:contrib-cbor
  - requirement:contrib-passkey
  - requirement:contrib-otel
  - requirement:contrib-reverse-proxy
  - requirement:contrib-jwt
  - requirement:contrib-oidc
  - requirement:contrib-database
  - requirement:contrib-html-template
  - requirement:contrib-zstd
principles:
  - policy:contrib-compatibility
  - decision:contrib-delivery-order
  - decision:passkey-first-authentication
acceptance: requirement:contrib-acceptance
scope_rule: implement explicit useful subsets; never claim full upstream compatibility without conformance evidence
```
