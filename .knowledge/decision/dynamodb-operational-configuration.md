---
id: decision:dynamodb-operational-configuration
type: decision
title: A Table's Operational Configuration Belongs To Deployment Tooling
---
Schema application owns whether a table exists and what its keys are; TTL, retention, and everything else about how a table is operated is defined by CloudFormation or its equivalent, and the framework touches none of it.

```yaml
status: accepted
decided: user 2026-08-01, over TTL, and the reasoning covers the rest of the surface
premise: a production DynamoDB table is defined by deployment tooling, so a framework that also defined it would be a second author of one resource
framework_owns:
  - whether the table exists
  - the partition key, the sort key, and their attribute types
  - agreeing with what is deployed, per requirement:dynamodb-migration
framework_never_owns:
  - TTL, including enabling it and reporting whether it is on
  - point-in-time recovery, backup, and retention
  - autoscaling and reserved capacity
  - tags, encryption keys, and deletion protection
  - global tables and replication
  mostly_already_true: system:tinygodriver-dynamodb excludes almost all of these, so this decision names an existing boundary rather than drawing a new one
billing_and_capacity:
  outcome: no configuration key, decided 2026-08-01
  reasoning: creation happens in development and test, where the target is an emulator that ignores both, so a key to set them would configure nothing
  created_with: the driver default, on-demand billing
  not_compared: reporting a difference would fire on every correct production deployment, since deployment tooling legitimately provisions differently
attributes_the_framework_still_maintains:
  what: a record field a deployment's TTL configuration points at, such as the dead_at of requirement:dynamodb-session-store
  why: the value is per-record and only the writer can compute it; the policy over that value is the deployment's
  boundary: the framework guarantees the attribute is correct, and never that anything acts on it
documentation_duty:
  rule: where the framework maintains such an attribute, the guide states what a deployment must configure for it to have any effect
  why: an attribute nothing acts on looks like a working feature, and silence would make retaining every record forever the default outcome
withdrawn_upstream_ask:
  what: UpdateTimeToLive and DescribeTimeToLive, previously ranked first in system:tinygodriver-dynamodb because they unlocked the session backend
  now: not wanted here; the framework would not call them if it had them
  still_wanted_elsewhere: system:tinybind has its own TTL tag requirement, which is a declaration rather than an apply step, so it is unaffected
related:
  - requirement:dynamodb-migration
  - decision:dynamodb-desired-state-migration
  - decision:dynamodb-session-expiry
  - policy:dynamodb-migration-safety
```
