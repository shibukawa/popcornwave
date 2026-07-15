---
id: decision:contrib-delivery-order
type: decision
title: Contrib Delivery Order
---
Contrib packages ship in dependency and feasibility order rather than as one release-sized feature.

```yaml
status: proposed
phases:
  - phase: 1
    packages:
      - requirement:contrib-httpmux
      - requirement:contrib-cbor
      - requirement:contrib-jwt
      - requirement:contrib-reverse-proxy
      - requirement:contrib-otel
  - phase: 2
    packages:
      - requirement:contrib-passkey
      - requirement:contrib-oidc
      - requirement:contrib-html-template
      - requirement:contrib-zstd
  - phase: 3
    packages:
      - requirement:contrib-postgresql
      - requirement:contrib-mysql
  - phase: feasibility-gated
    packages:
      - requirement:contrib-sqlite
rationale:
  - HTTP routing parity is required by decision:stdlib-servemux and unblocks generated method-and-path routes on TinyGo
  - requirement:contrib-passkey follows requirement:contrib-cbor and decision:passkey-first-authentication
  - OIDC depends on JWT and HTTP
  - database wire protocols require larger interoperability matrices
  - SQLite depends on target-specific C integration or a separately proven alternative
```
