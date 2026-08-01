---
id: flow:step-up-reauthentication
type: flow
title: Step-Up Reauthentication
---
An operation whose assurance requirement is unmet parks its intent, re-proves the same identity, and resumes.

```yaml
preconditions:
  - an authenticated session exists; an anonymous request goes to ordinary login instead
  - api:assurance-guard reported proof older than the age the operation named
status: implemented over the existing login and callback pair rather than as new endpoints, because both already own the provider round trip
implemented:
  entry: the guard redirects to the login path carrying the window and the return path
  marker: the existing transaction cookie carries whether this is a re-proof and the window it must satisfy, so the requirement a login started with is the one its callback enforces rather than one the callback reads from a query
  request: max_age and prompt=login, with CallbackOptions.RequireAuthTime set, so a provider answering silently from its own single sign-on session is refused
  same_identity: issuer, key claim, and key are compared against the session before anything is written
  admission: policy:oidc-admission is re-evaluated, so an identity that lost it fails the re-proof instead of refreshing it
  rotation: RotateWithMethod preserving the account and payload, updating ProviderAuthTime and, for a zero window, StepUpAt
  untrusted_query: the window in the redirect is not trusted; a larger value only weakens the request, and the guard re-evaluates its own requirement when the browser returns
flow:
  - park the pending operation as an opaque expiring requirement:contrib-auth-state record, keyed by a short-lived strict same-site cookie
  - answer with the response the api:assurance-guard wrapper selected at registration: a redirect for a page route, or 401 with a problem document for an API route
  - run the configured proof: requirement:contrib-oidc authorization carrying max_age and optional prompt=login, or a passkey assertion through api:passkey-endpoints
  - verify the result and the same-identity check of policy:reauthentication
  - rotate the session with the new data:session-assurance-state
  - consume the parked record once and return to the original operation, which is also the admission a zero window is satisfied by, per the zero_semantics of api:assurance-guard
failure:
  identity_mismatch: discard the parked record, keep the session, and report a generic failure
  provider_error: discard the parked record and return a generic unauthorized response
  expired_park: land on a safe path rather than replaying an operation the user has forgotten
rules:
  - the parked record holds a rooted same-site path and application-owned opaque intent, never an unbounded request body
  - the return target is validated exactly as api:authentication-endpoints validates a login return path, so step-up cannot become an open redirect
  - the parked record is single use and is spent whether the resumed operation succeeds or fails
  - a state-changing resumption still passes policy:csrf-protection against the rotated session
  - no ceremony state, ID Token body, or provider secret enters the parked record
  - the parked cookie is scoped to the re-proof path, matching the OIDC transaction cookie of api:authentication-endpoints
```
