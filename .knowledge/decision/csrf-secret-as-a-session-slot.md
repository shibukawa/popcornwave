---
id: decision:csrf-secret-as-a-session-slot
type: decision
title: The CSRF Secret Is a Session Slot
---
One registered session.Private slot holds the CSRF secret for every visitor, signed in or not, because the reason the two populations were split stopped being true.

```yaml
status: accepted
state: implemented
supersedes: decision:anonymous-csrf-secret-storage
was:
  authenticated: a csrf_secret field on data:session-record
  anonymous: a separate signed cookie written by its own middleware, under its own secret and its own opt-in flag
  reason_given: minting a stored session for every anonymous visitor lets unauthenticated traffic decide how many rows the store holds, so a crawler rather than the user base sets the count
why_that_reason_lapsed:
  fact: decision:slot-declared-placement made a session.Private slot ride a sealed cookie while the session is anonymous, whatever backend the deployment configured
  consequence: an anonymous visitor who is issued a secret writes no server row, which is exactly the cost the split existed to avoid
  therefore: the premise was answered by the storage model rather than needing a second mechanism
now:
  slot: one session.Private slot, declared by pw only where security.csrf.enabled is on
  anonymous: the secret rides the sealed cookie of the anonymous phase; no record, no separate keyring, no separate opt-in
  authenticated: the login rotation moves the same slot onto the configured backend
  rotation: the slot declares session.ResetOnRotate, so a login mints a fresh secret and a token minted before a sign-in cannot be presented after one
  destruction: a logout destroys it with the session, which is what data:session-record already promised
  verification: unchanged; the token derivation, the mask, and the origin check are the ones already implemented
removed:
  - the anonymous CSRF middleware and its cookie
  - security.csrf.anonymous and its own secret and previous_secrets keys
  - the csrf_secret field on data:session-record
  - the CSRF cookie name on the session cookie policy, which moved to security.csrf.cookie_name because writing that cookie is the check's own job
gains:
  one_path: everything after obtaining the secret was already identical; now obtaining it is identical too
  one_secret: the session keyring seals it, so a deployment configures no second signing key
  no_opt_in: a public page carrying an unsafe form needs no extra flag, because the same slot serves it
  revocation: a logout revokes the authenticated secret through the record, which a cookie-held one could not
costs:
  issuance: a page carrying a form issues a token cookie to an anonymous visitor, so the lazy issuance of api:session-registry stops being free on those pages; it still writes no server row
  registration: the slot is declared at setup rather than from an init, because a project with the check off must not be forced into a keyring on its account
rules:
  - the secret is never in the request view; only pwruntime carries it, to the code that verifies
  - the companion cookie carries a masked token and is never HttpOnly, because the browser runtime reads it
  - a lost companion cookie is rewritten from the held secret, which keeps the pair self-healing after a rotation
```
