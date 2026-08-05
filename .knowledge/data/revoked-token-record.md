---
id: data:revoked-token-record
type: data
title: Revoked Token Record
---
One entry of the policy:token-revocation list, holding enough to refuse a token and to explain the refusal later.

```yaml
table: popcornwave_auth_revocation, created by the rule:framework-owned-tables migration api:authentication-endpoints already publishes
fields:
  issuer: the authorization server the entry is scoped to, so the same subject value at another issuer is a different person
  kind: token or subject
  key_value: the jti for a token entry, and the auth.jwt.identity_claim value for a subject entry
  revoked_at: the stamp; for the subject form it is also the iat boundary below which a token is refused, so the two are one column rather than two that can disagree
  expires_at: revoked_at plus auth.jwt.max_token_lifetime, which is how long a token the entry must refuse could still be presented
  note: optional bounded operator text
primary_key: issuer, kind, key_value
plaintext_key:
  decision: the identifier is stored as written rather than hashed
  first_draft: a key_hash column, so the list would not itself be worth stealing
  why_reversed:
    sibling: popcornwave_auth_allowlist sits in the same migration and stores issuer, claim, and value in the clear, so hashing here would make two tables holding the same kind of value disagree about whether that value is a secret
    operability: an operator revokes, checks, and reinstates by hand during an incident, and a hashed column makes the list unreadable exactly when someone needs to read it
    threat: the values are already in every token the issuer mints and in the allowlist beside it; a database an attacker can read has already given them more than this table adds
excluded:
  - the compact token and any of its segments
  - any claim beyond the identifier the entry is keyed by
rules:
  - a second revocation of the same key moves revoked_at and expires_at forward rather than failing, because revoking twice is what an operator does when the first attempt did not obviously work
  - a lookup returns presence and revoked_at only; nothing in the record reaches a response body
  - note is operator text and is never rendered to a client
  - an entry is swept once expires_at has passed, per the expiry rule of policy:token-revocation
  - removing an entry is reinstatement rather than an undo: every unexpired token it was refusing works again at the next request
```
