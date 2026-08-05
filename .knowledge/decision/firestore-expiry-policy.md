---
id: decision:firestore-expiry-policy
type: decision
title: Expiry Is Decided On Read, And Deletion Is A Field Policy Nobody Here Applies
---
The Firestore stores never depend on an entity being deleted; correctness comes from the stored deadline checked on read and from the write precondition, and physical removal is a TTL policy the deployment configures on a timestamp property.

```yaml
status: accepted
decided: 2026-08-05, on the reasoning decision:dynamodb-session-expiry already settled for the same two questions
two_questions:
  correctness: is this record still valid
  storage: when do the bytes go away
  same_split_as_dynamodb: the relational store answers both with one DELETE, and on both non-relational stores they are unrelated
correctness:
  read: a record whose stored deadline has passed is reported as not found, whatever the entity says
  write: the renewal carries WithUpdateTime from the read, so it cannot apply to an entity that was rotated or removed in between
  authority: policy:session-security, where server-side expiry outranks anything the browser presents
  independent_of_deletion: TTL deletion happens within about 24 hours of expiry, so a store that trusted deletion would serve expired records for a day
storage:
  mechanism: a Firestore TTL policy over an ordinary timestamp property
  applying_it: deployment tooling, with gcloud firestore fields ttls update, per decision:dynamodb-operational-configuration, whose reasoning transfers unchanged
  what_the_framework_does: maintain one timestamp property per expiring kind so a deployment has exactly one correct property to point a policy at, and nothing else
  never: enabling a policy, reading whether one is enabled, or reporting its absence
what_differs_from_dynamodb_ttl:
  not_an_attribute_on_the_item: it is a field-level policy on a kind, so the unit of configuration is the kind rather than the table
  timestamp_not_epoch_seconds: the property must be a real Datastore timestamp, so the second epoch-numeric attribute requirement:dynamodb-session-store maintains beside its millisecond deadline has no counterpart; one datastore.Time property serves both the policy and the read check
  per_entity_opt_out: an absent or null property disables expiry for that entity, which is a property of the mechanism rather than a feature the stores use
  bounds: one TTL property per kind, and at most 500 policies per database, which four framework kinds do not come close to
  concurrency_mode: Datastore mode cannot combine TTL with a concurrency mode of Optimistic With Entity Groups, which is a database-creation choice and therefore a documentation duty rather than a check
  not_blocked_upstream: unlike system:tinygodriver-dynamodb, nothing was ever missing from the driver here; TTL is not on this wire at all
no_sweep:
  rejected: a Prune that queries for expired entities, as the relational backends publish
  why: a keys-only query over expired entities is cheaper than a DynamoDB Scan and still costs a read per entity, against a policy that removes them for nothing
  consequence_accepted: without a policy the kind grows without bound, which is a storage bill rather than a correctness fault
  which_stores: sessionstore/firestore, authstate/firestore, and the bootstrap kind of authstore/firestore; the credential and allowlist kinds have no expiry at all
documentation_duty:
  what: the guide names each kind, the timestamp property to point a policy at, and the gcloud command, beside the kind list decision:firestore-no-schema-application has the CLI print
  why_it_must_be_loud: a property the record maintains and nothing acts on looks like a working feature, and the default outcome of silence is retaining every record forever
  where_the_property_name_comes_from:
    contract: the firestorebind Expirer interface, ExpiryProperty() (string, bool), added in v0.3.6 on this repository's ask
    written_by_a_tag: a firestore ttl tag option for a generated type, and by hand for these five stores, which are handwritten
    what_it_replaces: a list of property names maintained beside the types, which drifts on a rename with no compile error and no runtime error - only a policy pointing at a property that no longer exists
    changes_no_bytes: the property is written as an ordinary timestamp; the interface is a declaration for the publishing step and does nothing on the write path
related:
  - requirement:firestore-session-store
  - requirement:contrib-auth-state-firestore
  - requirement:firestore-auth-stores
  - decision:dynamodb-session-expiry
  - decision:dynamodb-operational-configuration
```
