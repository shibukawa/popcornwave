---
id: requirement:firestore-auth-stores
type: requirement
title: Firestore Account-Side Authentication Stores
---
authstore/firestore implements the allowlist, credential, and bootstrap stores of plugin/auth over requirement:firestore-store; the fourth store, ceremony state, is requirement:contrib-auth-state-firestore.

```yaml
package: authstore/firestore
family: authstore, per rule:storage-package-layout
why_one_package: decision:firestore-conditional-writes commits two of these stores' writes together, so they ship and are verified together, which is the same reason requirement:dynamodb-auth-stores gives for its own grouping
selected_by: decision:auth-backend-selection
client:
  source: the request context, installed by the api:firestore-package middleware
  no_constructor_argument: matching requirement:contrib-auth-state-firestore
  missing: firestorebind's ErrNoClient, surfaced as the contract error of each store
kinds:
  names: popcornweb_auth_allowlist, popcornweb_passkey_credential, and popcornweb_auth_bootstrap, unchanged from rule:framework-owned-tables
  literal: no resolution step, per decision:firestore-namespace-isolation
  handwritten: the entity mappings are written rather than generated, for the reason requirement:dynamodb-session-store gives; this package is the only reader and writer
  no_creation_step: nothing creates or verifies a kind, per decision:firestore-no-schema-application
  no_conditional_verification: the rule:framework-owned-tables conditional check exists because a deployment refused for a table it will never write to learns to ignore the refusal; here there is no table check at all, so the rule has nothing to apply and the mode-driven distinction disappears
allowlist:
  contract: api:auth-allowlist-store
  key: datastore.NameKey over the issuer, the claim name, and the value joined
  why_joined: the same shape requirement:dynamodb-auth-stores chose, and for a reason that survives the port; here an ancestor path keyed on the issuer would put every login's admission read into one entity group rather than one partition
  lookup: one LoadAll over the candidate keys, whose three results keep found, missing and deferred apart; any found entry is a match
  bound: datastore.MaxLookupKeys, which the driver checks and answers ErrTooManyKeys for; the candidate set is far smaller
  deferred: a Deferred key is an incomplete answer and therefore an error rather than a non-match, per api:auth-allowlist-store, exactly as an unprocessed batch key is on DynamoDB
  listing: QueryKeysPage over the kind, one page per call, which is administrative rather than per-request
  writes: none; provisioning is administrator tooling
credential:
  contract: api:auth-credential-store CredentialStore
  key: datastore.NameKey over the credential ID
  find: one Get, strongly consistent by default, so a credential enrolled moments ago is findable on the next login with no second read and no consistency option
  list_by_account:
    operation: one QueryPage filtering account_id for equality
    index: none declared; single-property indexes are automatic, so the global secondary index requirement:dynamodb-auth-stores has to declare at creation has no counterpart
    what_this_removes: the bound where an index cannot be added to an existing table, and the deployment that has to recreate a table to get one
    unordered: the listing has no Order, because an order on a second property is exactly what would demand a composite index, per decision:firestore-no-schema-application
    projected: the credential ID, the transports, and the user handle
    why_the_handle: an enrollment reuses the handle the account already has, and a listing that could not see it would mint a second one for the same account
  save: Insert, which fails ErrAlreadyExists on a taken credential ID; inside the first-enrollment transaction it is one of two queued mutations
  update_on_assertion:
    operation: a transaction that reads the stored sign count and writes only when the accepted one is higher
    why_a_transaction: the predicate compares against a stored value and nothing on this wire evaluates one, per decision:firestore-conditional-writes
    why_conditioned_at_all: a counter that moves backward is a replayed or cloned authenticator, and the store refuses it rather than the caller
    failure: refused, and never downgraded to a warning
    whole_entity_rewrite: the transaction rewrites the credential entity, since there is no partial update; the entity is small and bounded
  delete: one Delete, idempotent by construction
bootstrap:
  contract: api:auth-credential-store BootstrapStore
  key: datastore.NameKey over the login ID
  issue: Insert, which fails on a live login ID without overwriting
  find: one Get, then the unconsumed, unexpired, and attempts-remaining checks
  record_attempt:
    operation: a transaction that reads the record and writes it back with one fewer attempt, only when it is unconsumed and has attempts left
    why_not_one_request: property transformations are excluded by the driver, so there is no server-side decrement to condition
    contract_still_met: two parallel guesses cannot both spend the last attempt, because the loser aborts and re-runs against the written record
    exhausted: reported as the contract's unknown-bootstrap error
  consume: queued inside the first-enrollment transaction beside the credential insert, per decision:firestore-conditional-writes
  enumeration: an exhausted budget, a consumed credential, and an unknown login ID all return the same error, unchanged from the contract
first_enrollment:
  shape: one transaction carrying the bootstrap consume and the credential insert, then the activation callback outside it
  seam: auth.FirstEnrollmentStore, the optional interface decision:dynamodb-auth-compensating-registration added; this backend implements it to commit the pair rather than to order it
  partial_states: two, not three, and the surviving one is a stored credential on an account still provisional, which cannot create a session per data:user-account
  why_the_callback_stays_outside: the closure re-runs on ABORTED, and an activation that ran twice is a side effect the transaction cannot bound
stored_properties:
  timestamps: datastore.Time properties, since Datastore has a real timestamp type
  binary: credential IDs, user handles, COSE blobs, curve points, and secret digests are Unindexed Blob properties, both because nothing filters on them and because system:tinygodriver-firestore silently unindexes an over-long value rather than refusing it
  account_id: indexed, because the credential listing filters on it; the only property in this package that needs an index
  expires_at: the bootstrap entity carries one, and it is the property a TTL policy points at
  expiry_authority: the stored deadline checked on read, per decision:firestore-expiry-policy
  no_ttl_here: nothing applies or verifies a policy; the credential and allowlist kinds have no expiry at all
bounds:
  entity_size: a record over datastore.MaxEntityBytes is refused before the request with the limit named
  key_size: a joined allowlist key carries an issuer and a claim value, which is the one key here that a long input could push toward datastore.MaxKeyBytes; it is bounded before the key is built
  transports_and_label: bounded before encoding, since an unbounded label is the only credential field an application controls
security:
  - no request this package issues produces a query-diagnostics record, per policy:query-log-safety
  - errors carry no credential ID, user handle, login ID, secret, or claim value, per policy:passkey-security and policy:oidc-security
  - the raw bootstrap secret is never stored or logged, per data:account-bootstrap-credential
acceptance:
  - a passkey login and a passkey enrollment onto an active account complete with no relational database configured
  - oidc.admission registered admits a pre-registered identity and denies an unregistered one, and a deferred lookup key is an error
  - a second Issue for a live login ID fails without overwriting
  - N parallel RecordAttempt calls against a budget of one leave exactly one caller with an attempt
  - an assertion carrying a sign count at or below the stored one is refused by the store
  - a first enrollment interrupted before the commit leaves neither a spent bootstrap record nor a credential
  - a first enrollment interrupted after the commit leaves a provisional account that cannot create a session
  - the credential listing returns every credential of one account with no index declared and no composite index applied
  - two parallel test servers with different namespaces never observe each other's credentials, per requirement:firestore-test-isolation
implemented:
  built: 2026-08-05, in authstore/firestore
  one_commit: the bootstrap consume and the credential insert share a transaction, reached by carrying the open transaction on the context the framework already threads through its spend callback; the bootstrap store joins it when one is there and opens its own when it is not
  verified: an interrupted first enrollment leaves neither write, and one issued secret cannot enroll two authenticators
non_goals:
  - a sweep of expired bootstrap records
  - storing data:user-account or data:external-identity, which stay application-owned
  - sharing one kind with requirement:contrib-auth-state-firestore or requirement:firestore-session-store
  - an ordered credential listing, which would need a composite index nothing applies
  - an ancestor relation making an account the parent of its credentials, which would serialize writes across one account's enrollments
related:
  - requirement:firestore-auth-backend
  - requirement:contrib-auth-state-firestore
  - api:firestore-package
```
