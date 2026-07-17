---
id: data:auth-state-record
type: data
title: Authentication State Record
---
Durable authentication state stores persist one namespaced, expiring, single-use record without exposing its secret payload.

```yaml
fields:
  namespace: bounded non-empty deployment and protocol scope
  key: bounded opaque correlation key
  expires_at_ms: absolute Unix millisecond deadline
  payload: bounded api:auth-state-codec bytes
semantics:
  put: create only while no unexpired namespace and key record exists
  take: atomically remove before expiry validation, decode, or return
  expired: remove and return requirement:contrib-auth-state ErrExpired or ErrNotFound
  malformed: remain consumed and return a stable decode error
security:
  - namespace, key, expiry, and payload never enter logs or stable error text
  - storage and transport protections match the deployment threat model
  - payload encryption is an explicit codec or deployment responsibility
```
