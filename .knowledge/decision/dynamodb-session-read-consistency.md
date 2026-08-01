---
id: decision:dynamodb-session-read-consistency
type: decision
title: Eventually Consistent Reads, With A Consistent Retry On A Miss
---
A session read is eventually consistent by default, and a miss is retried once with a consistent read, so the cheap path stays cheap and the read-your-own-login case cannot fail.

```yaml
status: accepted
decided: user 2026-08-01, eventual by default
cost:
  eventually_consistent: half the read capacity of a consistent read
  shape_of_the_workload: one session read on nearly every authenticated request, so the default is the bill
the_hazard:
  case: login writes a rotated session, the browser follows the redirect, and the next request lands on a node whose replica has not caught up
  symptom: the item exists and is not returned, so the user appears logged out immediately after logging in
  frequency: rare, usually milliseconds wide
  why_it_still_matters: it is unreproducible, it looks like an authentication bug, and policy:session-security rotates the session at exactly that moment
the_retry:
  rule: a read that finds nothing is retried once with a consistent read, and its answer is final
  a_hit_is_never_reread: a returned item is accepted as it stands, so the common authenticated request pays the cheap read alone
  why_this_is_enough: the hazard is a false miss, and a consistent read cannot produce one
  cost_of_a_real_miss: a stale or forged cookie pays a second read and then does no session work at all, so the extra capacity is bounded by requests that were going to be rejected
  not_a_loop: exactly one retry; a consistent read that also finds nothing means the record is not there
override:
  key: data:session-runtime-config session.dynamo.consistent_read
  effect: true makes the first read consistent and removes the retry, since there is nothing left for it to catch
  when: a deployment that would rather pay double than reason about replica lag at all
rejected_alternatives:
  consistent_by_default: correct and twice the price on every request, which the retry buys back
  retry_with_a_delay: a sleep on the request path to wait out replication, which trades a rare wrong answer for a routine slow one
  sticky_routing: making the redirect land on the writing node, which is a deployment constraint the framework cannot impose
scope: reads only; every write and the Touch condition are already strongly consistent, because DynamoDB evaluates a condition on the leader
related:
  - requirement:dynamodb-session-store
  - decision:dynamodb-session-expiry
  - policy:session-security
```
