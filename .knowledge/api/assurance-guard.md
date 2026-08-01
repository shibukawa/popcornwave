---
id: api:assurance-guard
type: api
title: Assurance Guard
---
An application declares one thing about assurance, that an operation needs recent proof, and the framework supplies the challenge and the way back.

```yaml
package: github.com/shibukawa/popcornwave/plugin/auth
status: implemented, except the strength requirement deferred below
implemented:
  requirement_sources: auth.MaxAge, auth.Policy, auth.Dynamic, and auth.Default, behind one auth.Requirement interface
  wrappers: auth.Ensure redirects and auth.EnsureAPI answers 401
  escape_hatch: auth.IsRecent and auth.Challenge
  reader: auth.LastProvedAt
  config: the auth.assurance.policy array of tables, validated for a missing name, a duplicate name, and a negative window
  zero_window: SessionData.StepUpAt, set by a completed step-up and bounded by a short admission window
  existing_use: the api:passkey-endpoints enrollment check now calls auth.IsRecent and auth.Challenge, so its refusal names what was missing instead of being a bare 403
  freshness_source: SessionData.ProviderAuthTime when the provider reported auth_time, and the session AuthenticatedAt otherwise
application_states:
  authenticated: policy:authenticated-path-protection, declared by path in configuration and needing no handler code
  recently_proved: this concept, declared per operation in the handler that performs it
  reason: an application developer branches on these two and on nothing else; every other level of concept:assurance-axes is configuration or framework-internal
form:
  shape: a handler decorator applied where the route is registered, not a middleware
  reason: pw.Middleware is applied by App.Use across the whole application, and App.Handle takes one handler with no per-route middleware slot, so the registration site is the only place a per-route requirement can be stated
  benefit: the requirement sits beside the pattern it guards, so a route cannot be added without deciding it
surface:
  - auth.Ensure(handlerFunc, maxAge) wraps a page route and answers an unmet requirement with a redirect
  - auth.EnsureAPI(handlerFunc, maxAge) wraps an API route and answers with 401 and a problem document
  - auth.IsRecent(r, maxAge) reports whether the requirement is met and writes nothing
  - auth.Challenge(w, r, maxAge) writes the challenge and starts flow:step-up-reauthentication
  - auth.StepUp(w, r, maxAge, intent) parks an application-owned opaque intent, for a mutation that must resume
  - auth.LastProvedAt(context) reports effective freshness, so a page can warn before the user commits to a long form
split_predicate_and_challenge:
  reason: a function that both returns a decision and writes a response hides control flow, so the escape hatch is two calls
  use: a threshold that depends on the request, such as a stricter window above a payment amount
signature:
  takes_and_returns: http.HandlerFunc, so one name serves both App.Handle and App.HandleFunc and a function literal needs no http.HandlerFunc conversion
  requirement_type: the second parameter is an auth.Requirement rather than a time.Duration, because Go has no overloading and the sources of a window are plural
requirement_sources:
  - auth.MaxAge(d) states the window literally, for a route whose window is a property of the code
  - auth.Policy(name) reads the window from auth.assurance.policy.<name>, so a deployment tunes user experience without a rebuild
  - auth.Dynamic(func(*http.Request) time.Duration) computes it, for a wall-clock deadline that no fixed duration expresses
  extensibility: one parameter type absorbs each source without adding a wrapper name
named_policy:
  config: auth.assurance.policy.<name>.max_age
  validation: a Policy naming an undefined entry fails startup rather than at request time
  purpose: the same handler code serves a consumer deployment with a long window and an internal one with a short window
  reuse: one name applied across many routes keeps them consistent, which a literal repeated at each registration does not
wall_clock_windows:
  case: an internal system requiring re-confirmation after every midnight, which is a deadline rather than a duration
  form: auth.Dynamic returning the elapsed time since the most recent boundary, so the proof must postdate it
  reason: max_age is defined as elapsed seconds, so a deadline is expressed by computing the elapsed value at request time
registration_example:
  read: App.HandleFunc("GET /payment", auth.Ensure(page, auth.MaxAge(30*time.Minute)))
  write: App.HandleFunc("POST /api/payment", auth.EnsureAPI(charge, auth.MaxAge(45*time.Minute)))
  area: App.HandleFunc("GET /admin", auth.Ensure(adminPage, auth.Policy("admin")))
  operation: App.HandleFunc("POST /api/admin/drop", auth.EnsureAPI(drop, auth.Policy("danger")))
  boundary: guarding the read is user experience, because a client can post directly; the mutation is the security boundary and needs its own guard
  asymmetry: the write window is the more generous one, so a user who reads, fills a long form, and submits does not lose the input to a guard the read had just satisfied
  resumption: a mutation that must survive the step-up passes an application-owned intent to auth.StepUp, because flow:step-up-reauthentication refuses to park a request body
max_age:
  omitted: auth.recent_auth_max_age, the existing default
  positive: the recent level of concept:assurance-axes, such as entering an administration area
  zero: the per-operation level, such as a destructive action inside an area the user already entered recently
  two_levels_one_predicate: entering an area and performing a dangerous action inside it are the same check with different windows, so the guard needs one predicate rather than two strengths
  guidance: the window is the expressive part; a deployment tunes it, and no new level is needed to say a stronger thing
zero_semantics:
  hazard: evaluating zero as elapsed time since the proof never converges, because the redirect to the provider and back always consumes more than zero seconds, so the guard challenges again immediately after a successful re-proof
  rule: zero is satisfied only by a single-use admission that a completed flow:step-up-reauthentication round trip issues, never by a timestamp comparison
  implemented: a StepUpAt stamp written into the session at the completed step-up, admitted while it is within a short window
  why_the_session: a cookie would be forgeable by anything that can set one, and the session is the only unforgeable place this package can write without owning a new server-side table
  shortfall: the stamp is time-bounded rather than consumed exactly once, because consuming it would mean rotating the session inside an ordinary read, so two zero-window operations inside the window share one proof
  follow_up: a server-side single-use record restores exact per-operation semantics
resumption_limit:
  fact: the callback returns through a redirect, which is a GET, so the wrapper can resume only a safe method
  consequence: a guarded mutation cannot be replayed by the wrapper; it answers the challenge and the user arrives back on the page that submitted it
  escape: auth.StepUp carries an application-owned intent for a mutation that must survive, per flow:step-up-reauthentication
  pairing: this is why the read route carries the real window and the write route carries the boundary, so the write guard is rarely the one that fires
placement:
  order: after session and authentication middleware, and after policy:authenticated-path-protection
  reason: an anonymous request is a login problem rather than an assurance problem, so the guard never sees one
response:
  selection: the wrapper chosen at registration, never negotiated from a request header
  redirect: auth.Ensure sends the browser into flow:step-up-reauthentication
  reject: auth.EnsureAPI answers 401 with a problem document naming the unmet max_age, so a client can start the step-up itself
  status: 401 rather than 403, because the remedy exists and the client is meant to re-prove and retry, which is also what RFC 9470 returns
  header: the RFC 9470 insufficient_user_authentication challenge is emitted only for a Bearer-protected endpoint, because that challenge belongs to the Bearer scheme and a cookie-authenticated route is not in it
  existing_403: the current api:passkey-endpoints refusal is 403, which is honest only while no remedy exists; adding the step-up path makes 401 the correct code
no_negotiation:
  rejected: choosing between redirect and reject from Accept or Sec-Fetch-Mode
  asymmetry: misreading an API call as a page returns a redirect that an XHR follows transparently, so the client receives 200 and an HTML login page instead of an error, which fails silently; misreading a page as an API shows a raw 401 and fails loudly
  unreliable_signal: fetch defaults to Accept */* and so does HTMX, so the header is weakest exactly where the routes differ
  precedent: policy:authenticated-path-protection already settles the same question explicitly through protection.unauthenticated, and negotiating here would contradict it
  no_default: the behavior is two function names rather than an option with a default, because an API route that forgets the option would take the silent failure mode
existing_use:
  target: the api:passkey-endpoints register/begin freshness check becomes auth.Ensure with the same default
  effect: a refusal that is a dead end today gains a remediation path
deferred:
  strength_requirement:
    surface: auth.RequireStrength and the ordered auth.assurance.strengths list
    reason:
      - oidc_only and passkey_only each produce one method, so an ordering has nothing to order
      - oidc_passkey produces two, and neither is stronger in general; a provider with a hardware key outranks a local passkey without user verification, and the reverse also happens
      - an ordering is therefore a deployment claim, and no default the framework picks is defensible
    until: a deployment states an ordering the framework can carry rather than invent, most plausibly as acr through requirement:contrib-oidc
rules:
  - a requirement names assurance and never an issuer, a provider, or a ceremony
  - the guard reads assurance and never writes it; only policy:reauthentication writes it
  - a handler reading auth.LastProvedAt or auth.IsRecent without guarding owns its own denial
  - a decorated read route is not a boundary for its matching write route; each pattern carries its own requirement
  - the surface is identical across auth modes, so an application does not branch on oidc_only, oidc_passkey, or passkey_only
  - api:testutil-auth sets authenticated_at, so a test proves the admit path and the challenge path without a provider
```
