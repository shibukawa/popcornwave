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
      - requirement:contrib-auth-common
      - requirement:contrib-auth-state
      - requirement:contrib-cbor
      - requirement:contrib-jwt
      - requirement:contrib-reverse-proxy
      - requirement:contrib-otel
      - requirement:contrib-redis-valkey
  - phase: 2
    packages:
      - requirement:contrib-passkey
      - requirement:contrib-oauth
      - requirement:contrib-oidc
      - requirement:contrib-html-template
      - requirement:contrib-zstd
  - phase: deferred-non-primary
    packages:
      - requirement:contrib-postgresql
      - requirement:contrib-mysql
  - phase: feasibility-gated
    packages:
      - requirement:contrib-sqlite
rationale:
  - decision:stdlib-servemux uses the standard router provided by decision:tinygo-042-baseline
  - requirement:contrib-passkey follows requirement:contrib-cbor and decision:passkey-first-authentication
  - requirement:contrib-oauth depends on shared authentication state and security primitives
  - requirement:contrib-oidc extends requirement:contrib-oauth and depends on JWT
  - requirement:contrib-html-template remains phase 2 because frontend JSON writers require generated encoding work
  - database wire protocols require larger interoperability matrices
  - requirement:contrib-redis-valkey is bounded to session and lightweight shared-state commands through a local proxy
  - decision:server-sql-support-tier excludes server SQL drivers from first-class delivery
  - SQLite depends on target-specific C integration or a separately proven alternative
```
