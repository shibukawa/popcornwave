---
id: data:session-record
type: data
title: Session Record
---
A stored session record binds an opaque token hash to the private slots of api:session-registry and to the lifetime it was handed, and it states nothing about identity on its own.

```yaml
instances:
  count: one per placement a session's slots actually use, so at most a cookie-placed record and a server-placed record
  shared: the key hash, the timestamps, and the version, which are the session's rather than the record's
  reason: decision:slot-declared-placement lets slots choose, and a record cannot span two places
stored_fields:
  key_hash: hash of random cookie token
  slots: one immutable encoded value per server-placed slot in this record, keyed by its registration key
  created_at: timestamp
  last_seen_at: timestamp
  expires_at: absolute timestamp
  idle_expires_at: optional timestamp
  csrf_secret: not a field; the CSRF secret is a registered session.Private slot, per decision:csrf-secret-as-a-session-slot
  version: integer
not_stored_here:
  fields: authenticated_at and authentication_method
  location: the plugin/auth slot, beside the rest of data:session-assurance-state
  reason: concept:session-storage-boundary; a record holding a shopping cart for an anonymous browser has no authentication time to report
lifetime_fields:
  written_by: api:session-manager, from the durations decision:session-lifetime-owned-by-auth supplies
  meaning_to_the_store: a deadline to enforce, with no interpretation of what expires
request_view:
  - each registered slot, decoded to its own type and reachable only through api:session-registry
  - timestamps and expiry
  - version
excluded_from_request_view:
  - raw cookie token
  - key hash
  - backend serialization bytes
  - CSRF secret
  - any slot the reader did not register
rules:
  - store codecs are bounded and reject malformed records
  - a slot is encoded independently, so one unreadable slot is cleared rather than failing the whole record
  - a session.Private slot appears in the cookie-placed record while anonymous and in the server-placed one afterward, never in both
  - expiry is authoritative on the server even when a cookie remains
  - version supports invalidation after incompatible schema or policy changes
  - CSRF secret rotates with session creation and with the rotation of api:session-manager
  - a record with no lifetime source carries no expires_at, per decision:session-lifetime-owned-by-auth
implemented:
  shape: one typed payload T rather than a slot map, with authenticated_at and authentication_method as record fields
  migration: the payload becomes the plugin/auth slot, per decision:type-keyed-session-storage
```
