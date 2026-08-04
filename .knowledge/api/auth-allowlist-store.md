---
id: api:auth-allowlist-store
type: api
title: Admission Allowlist Store
---
The pre-registration lookup of policy:oidc-admission becomes a store seam, so it can be answered by something other than a SQL table.

```yaml
package: github.com/shibukawa/popcornwave/plugin/auth
problem:
  - the registered admission mode reads popcornwave_auth_allowlist through SQL built in plugin/auth, with no interface between
  - it is the only one of the four framework-owned authentication stores with no seam, so requirement:dynamodb-auth-backend cannot supply an implementation without one
  - an application that already knows who may enter, from a directory or an entitlement service, has no way to answer the question today
surface:
  - auth.SetAllowlistStore(store) installs the application implementation
  - Registered(ctx, issuer, candidates) reports whether any candidate matches, where a candidate is a claim name and its verified value
  - the framework passes every claim of auth.oidc.registered_claims that the verified identity carries, so the store answers one question per login rather than one per claim
why_a_batch_signature:
  sql: the current query is one statement with an OR over the compared claims
  dynamo: requirement:dynamodb-auth-stores answers it with one BatchGetItem
  a_per_claim_signature: would make both implementations issue N requests to answer one question, for no gain in expressiveness
entry:
  fields: issuer, claim name, expected value, and an operator note
  identity: the three key fields together, which is the primary key of the relational table
  provisioning: administrator tooling, outside this seam; the framework reads and never writes
default:
  when: no store is installed
  backing: popcornwave_auth_allowlist under rule:framework-owned-tables, selected by decision:auth-backend-selection
  override: installing a store means the framework creates and verifies no table for this capability, matching api:auth-credential-store
  unused_modes: nothing reads this store unless oidc.admission is registered, so the conditional verification of rule:framework-owned-tables applies to it as well
rules:
  - a lookup failure is an error and never a denial, so an outage cannot silently change who may enter, per policy:oidc-admission
  - matching is exact and case-sensitive, and the store performs no normalization
  - a claim the verified identity does not carry is omitted from the candidates rather than compared as empty
  - no candidate at all is a non-match rather than an error
  - store errors are logged without claim values, per policy:oidc-security
  - the store answers admission only, and never authorization
implemented:
  built: 2026-08-05, replacing the raw SQL in plugin/auth
  default: sqlAllowlist over popcornwave_auth_allowlist, one statement per login
  verification: the table is verified only under the registered admission mode and only when no store is installed
related:
  - policy:oidc-admission
  - data:external-identity
  - requirement:dynamodb-auth-backend
```
