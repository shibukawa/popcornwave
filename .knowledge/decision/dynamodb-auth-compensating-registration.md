---
id: decision:dynamodb-auth-compensating-registration
type: decision
title: Passkey-Only Registration Is A Compensating Sequence On DynamoDB
---
The DynamoDB auth backend cannot make flow:passkey-only-registration one unit of work, so it fixes an order whose every partial outcome is safe, and names the one state that needs an administrator.

```yaml
status: accepted
decided: user 2026-08-01
constraint:
  what: system:tinygodriver-dynamodb exposes GetItem, PutItem, UpdateItem, DeleteItem, Query, Scan, BatchGetItem, and BatchWriteItem, and no TransactWriteItems
  batch_is_not_atomic: BatchWriteItem can partially succeed by design, so it is not the missing primitive
  contrast: api:auth-credential-store Save opens a SQL transaction and runs the activation callback inside it, which is what makes the relational path one unit
  scope: only flow:passkey-only-registration and the recovery flow need three writes together; passkey login, passkey enrollment onto an active account, and every OIDC flow are single writes and are unaffected
rejected_alternatives:
  wait_for_transactions:
    what: rank TransactWriteItems up in system:tinygodriver-dynamodb and leave the credential and bootstrap stores unwritten
    why_not: it blocks requirement:dynamodb-auth-backend on an upstream item that decision:dynamodb-framework-scope already ranked third, behind two others
  drop_the_flow:
    what: support oidc_only and passkey login, and require a relational database for bootstrap registration
    why_not: passkey_only is the mode with no identity provider, which is exactly the deployment most likely to have no relational database either
order:
  1_spend: UpdateItem on the bootstrap record, conditioned on it being unconsumed, unexpired, and having attempts left, marking it consumed and returning the record
  2_persist: PutItem the credential, conditioned on that credential ID being absent
  3_activate: the activation callback, which promotes the provisional data:user-account and belongs to the application
  why_this_order: the bootstrap credential is the single-use authorization for the whole sequence, so spending it first makes the sequence itself single-use; a credential written before it could be written twice by two parallel redemptions
reachable_states:
  after_1: bootstrap spent, no credential, account still provisional
  after_2: bootstrap spent, credential stored, account still provisional
  after_3: complete
  never: a credential stored without its bootstrap credential being spent, which is the state that would let one issued secret enroll two authenticators
recovery:
  after_1: the account has no credential and no redeemable secret; an administrator issues a new bootstrap credential, which policy:account-recovery already covers
  after_2: the stored credential belongs to an account that cannot create a session, because data:user-account forbids a provisional account from doing so; the account is activated by retrying the callback or by the same administrator path
  named: this is a defined outcome of a failed registration rather than a corrupt state, and the guide says so
  no_compensation_rollback: the backend does not delete the credential or unspend the bootstrap record after a failure, because a rollback that itself fails leaves a state nobody specified
retry:
  same_secret: refused; the bootstrap record is consumed, so step 1 fails closed with the enumeration-safe error api:auth-credential-store already returns
  idempotence: step 2 is conditioned on absence, so a retried request that already stored the credential is a conditional-check failure rather than a second credential
divergence_from_the_relational_path:
  what: api:auth-credential-store states that a partially applied ceremony is a defect, not a recoverable state
  now: that holds for the relational default, and this backend narrows it to a bounded, enumerated set of partial states with a named operator action
  documented: the guide states the difference where auth.backend is described, because a deployment choosing this backend is accepting it
revisit_if:
  - system:tinygodriver-dynamodb gains TransactWriteItems, after which the three writes become one and this decision is superseded rather than amended
related:
  - requirement:dynamodb-auth-backend
  - requirement:dynamodb-auth-stores
  - api:auth-credential-store
  - flow:passkey-only-registration
  - policy:account-recovery
```
