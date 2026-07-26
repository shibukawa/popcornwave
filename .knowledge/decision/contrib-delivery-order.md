---
id: decision:contrib-delivery-order
type: decision
title: Contrib Delivery Order
---
Contrib packages ship in dependency and feasibility order rather than as one release-sized feature.

```yaml
status: proposed
prerequisite: requirement:tinygodriver-adoption
phases:
  - phase: 1
    packages:
      - requirement:contrib-auth-common
      - requirement:contrib-auth-state
      - requirement:contrib-auth-state-memory
      - requirement:contrib-cbor
      - requirement:contrib-jwt
      - requirement:contrib-otel
  - phase: 2
    packages:
      - requirement:contrib-passkey
      - requirement:contrib-oauth
      - requirement:contrib-oidc
      - requirement:contrib-devidp
      - requirement:contrib-html-template
      - requirement:contrib-auth-state-redis
  - phase: deferred-non-primary
    packages:
      - requirement:contrib-postgresql
      - requirement:contrib-mysql
  - phase: feasibility-gated
    packages:
      - requirement:contrib-auth-state-sqlite
rationale:
  - decision:stdlib-servemux uses system:tinygodriver rather than a contrib router
  - requirement:contrib-passkey follows requirement:contrib-cbor and decision:passkey-first-authentication
  - requirement:contrib-oauth depends on shared authentication state and security primitives
  - requirement:contrib-auth-state-memory is the process-local reference adapter for the base store contract
  - requirement:contrib-oidc extends requirement:contrib-oauth and depends on JWT
  - requirement:contrib-devidp follows requirement:contrib-oidc because it exists to exercise the relying party, and it ships with api:testutil-idp
  - requirement:contrib-html-template remains phase 2 because frontend JSON writers require generated encoding work
  - database wire protocols require larger interoperability matrices
  - requirement:contrib-auth-state-redis follows the base store and tested requirement:contrib-redis-valkey dependency
  - requirement:contrib-auth-state-sqlite follows the portable SQLite facade
  - SQLite and Zstandard implementations are supplied by system:tinygodriver
  - decision:server-sql-support-tier excludes server SQL drivers from first-class delivery
  - SQLite depends on target-specific C integration or a separately proven alternative
```
