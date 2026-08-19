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
      - requirement:contrib-jwt
      - requirement:contrib-otel
  - phase: 2
    packages:
      - requirement:contrib-passkey
      - requirement:contrib-passkey-test
      - requirement:contrib-oauth
      - requirement:contrib-oidc
      - requirement:contrib-devidp
      - requirement:contrib-html-template
      - requirement:contrib-auth-state-redis
  - phase: supplied-by-tinygodriver
    packages:
      - requirement:contrib-cbor
      - requirement:contrib-sqlite
      - requirement:contrib-postgresql
      - requirement:contrib-mysql
    note: these ship with system:tinygodriver, so the work here is consumption and acceptance rather than implementation; cbor shipped in phase 1 first and was upstreamed in v1.2.6
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
  - requirement:contrib-passkey-test follows requirement:contrib-passkey for the same reason and ships with api:testutil-passkey, because decision:passkey-test-authenticator makes it the only way to exercise a ceremony in CI
  - requirement:contrib-html-template remains phase 2 because frontend JSON writers require generated encoding work
  - database wire protocols require larger interoperability matrices, which is why they are consumed rather than written here
  - requirement:contrib-auth-state-redis follows the base store and tested requirement:contrib-redis-valkey dependency
  - requirement:contrib-auth-state-sqlite follows the portable SQLite facade
  - SQLite and Zstandard implementations are supplied by system:tinygodriver
  - decision:server-sql-support-tier makes the three SQL engines first-class together, so none of them gates the others
  - SQLite depends on target-specific C integration or a separately proven alternative
```
