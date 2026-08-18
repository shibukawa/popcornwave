---
id: decision:storage-bounded-session-record
type: decision
title: The Store Bounds Its Own Records
---
A session record lives for the shorter of two ceilings on different things: how long the store may hold bytes, and how long a proof of identity stays good.

```yaml
status: accepted
state: implemented
corrects: decision:session-lifetime-owned-by-auth, which was right that a proof expiry is authentication's statement and over-reached by leaving the store with no bound of its own
what_went_wrong:
  claim: with no authentication linked, a session would be bounded by the browser alone
  reality_on_a_server_backend: the sweep of api:session-store deletes rows whose expiry is past, and it reads a zero expiry as already past; the renewal statement matches on a future expiry and never matches a zero one
  consequence: a record with no deadline was not merely unbounded, it was unusable, so the absent bound was a correctness gap rather than an accepted risk
two_ceilings:
  session.retention:
    states: how long the store may hold one record
    owner: data:session-runtime-config, because it is a property of the table rather than of a login
    default: 720h
    required: positive for a server backend, refused at startup otherwise, naming what a record with no deadline does to the sweep
  auth.session.ttl:
    states: how long a proof of identity stays good
    owner: data:authentication-runtime-config, unchanged
  effective: the shorter; neither subsumes the other, and a zero on either side is that side declining to bound it
why_this_does_not_re_split_the_policy:
  original_reason: an absolute expiry, an idle expiry, and a re-proof window are three answers to one question, and splitting them across two files split one policy
  still_true: those three stay together under [auth]
  different_question: how long a table may carry abandoned rows is not one of them, so it is not the policy being split
sweep:
  moved: from popcornweb/plugin/auth to the framework session extension
  reason: the records are the framework's; a deployment with no login still writes them for a session.ServerOnly slot, and abandonment rather than logout is how most sessions end
  kept_in_auth: the single-use ceremony records of requirement:contrib-auth-state, which that package owns
rules:
  - a server backend with no positive retention fails startup rather than writing records nothing can read back
  - the cookie backend keeps nothing on a server, so it accepts an unbounded record and is bounded by the browser
  - the sweep runs wherever session storage runs, not wherever authentication runs
```
