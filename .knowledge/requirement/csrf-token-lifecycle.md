---
id: requirement:csrf-token-lifecycle
type: requirement
title: CSRF Token Lifecycle And Delivery
---
This framework owns where a CSRF secret comes from, how long it lives, and how it reaches a browser: minted with the session and kept in data:session-record, or carried in a signed cookie for a visitor without one, and emitted to the cookie, the header, and the rendered form as values freshly derived from it.

```yaml
source: user CSRF lifecycle decision 2026-08-03
built: all three channels, the secret and its derivation, the middleware, the anonymous path of decision:anonymous-csrf-secret-storage, and the api:cli-init section
built_note: the header channel did not wait on requirement:unified-update-runtime after all; the boundary runtime that already ships issues a credentialed request of its own for the live delivery stream, so there was a consumer before the merged asset existed
division: system:tinybind writes the token into markup and reads it off a request per requirement:module-native-csrf; everything about where the value comes from, how long it lives, and how it reaches the browser is here
lifecycle:
  created: with the session, which for an authenticated deployment means at login
  stored: the csrf_secret field data:session-record already carries, never in the request view and never in a log
  rotated: with the session, so login rotation and an authentication-strength change carry the token with them, per policy:session-security
  destroyed: with the session at logout or revocation, along with the cookie that carried it
  scope: one secret per session; the value that reaches a browser is derived from it per response, per the derivation below
  without_a_session: decision:anonymous-csrf-secret-storage supplies the secret from a signed cookie instead, and nothing below changes
delivery:
  cookie:
    purpose: the channel the browser runtime reads when it issues a request, so a rotation reaches a page that is already open
    written: by the session manager, wherever it writes the session cookie itself, which is creation, rotation, and idle renewal; cleared wherever it clears that one
    why_there: pairing the two writes at one call site is what makes them impossible to diverge, so a browser holding a session cookie always holds a matching token
    value: a masked token rather than the stored secret, since the secret is what verification compares against and script can read this cookie
    readable_by_script: required, since the runtime must read it; this is the one cookie of policy:cookie-value-protection that is not http-only
    attributes: same-site lax, secure outside development, path-scoped to the application root, and an expiry tracking the session
    not_double_submit: for a session the cookie is transport only and verification reads the session's own secret; for the anonymous path the cookie is the secret and is authenticated by its signature, which is the variant policy:csrf-protection permits rather than the naive one it forbids
  header:
    what: the runtime attaches the value it read from the cookie to every request it issues
    read_at: the moment the request is made rather than page load, which is what makes rotation reach an open page
    name: the module default, deliberately outside the framework's header namespace because it is a name middleware already looks for
    one_helper: every request the runtime issues goes through a single function that attaches the token, so a call site cannot omit it by being written without it
  rendered_html:
    what: the token is supplied to the render and system:tinybind puts it in every unsafe form
    supplied_as: a render option, since htmlbind holds no context key and cannot read one
    read_from: api:request-context-accessors, where the session middleware already placed it
    selection_rule: a request carrying a session renders with its token; one without supplies no option at all, so an unsafe form fails the render rather than emitting an empty field, per the correction in requirement:module-native-csrf
  one_secret: the three channels carry different bytes that all unmask to one secret, so no channel can disagree about what session it belongs to
cookie_name_is_not_configurable:
  decision: the companion cookie's name is a constant shared by the Go side and the shipped script, not a setting
  why: the runtime is a module script, and a module has no document.currentScript, so it cannot read a name off its own tag the way a classic script can
  consequence_if_it_were: a renamed cookie would leave the runtime looking for one that is not there, and nothing would report it
  revisit: the merged asset of requirement:unified-update-runtime gains a configuration channel on its tag, at which point this can become a setting again
verification:
  entry: the module reads the header first, then the body field, and compares in constant time against what this framework supplies
  supplied_expected: the value recomputed from the incoming token's pad, not the stored secret
  failure: 403 through api:error-renderer without reaching the application handler, per policy:csrf-protection
  origin: the origin and referer checks of that policy still run, since a token alone is not the whole defence
derived_token:
  correction: an earlier reading of this requirement concluded that masking was incompatible with the module and dropped it; that was wrong, and the mistake was assuming the stored secret had to be the compared value
  what_is_actually_fixed: the module compares two strings in constant time and reads the incoming one header-first then body; both the emitted value and the compared value are this framework's to choose
  emission:
    pad: fresh random bytes per response, never reused and never stored
    value: the pad followed by the pad combined with the secret, encoded as one opaque string
    per_response_not_per_form: the module produces the token once per render and every form shares it, which is exactly the granularity a compression oracle needs defeating; a second form does not need a second pad
  verification:
    read: the incoming token through the module's own channel-order entry, so header-before-body stays in one place
    recompute: take the pad from the incoming token and build the value a correct token carrying that pad would have
    compare: hand that recomputed value to the module as the expected one, so the constant-time comparison and the missing-token result stay the module's
    malformed: a token of the wrong length or encoding is a mismatch rather than a panic, decided before anything is split
  why_it_is_safe: the pad is public and the combination is not invertible without the secret, so an attacker choosing a pad still has to produce the secret combined with it
  alternative_shape: this framework could compare directly and use the module only to read the token; recomputing the expected value instead keeps the channel order and the missing-token semantics from being restated
what_travels_masked:
  hidden_field: a fresh value on every response, which is the whole point
  cookie: one value written when the session is created or rotated, since a cookie is not in the compressed body and re-masking it per response would buy nothing and cost a set-cookie on every response
  header: whatever the runtime read from the cookie, relayed opaquely
  runtime_knows_nothing: the browser half never decodes or reconstructs anything, so masking costs no client change
  all_three_verify_alike: every channel carries a value that unmasks to the one secret, so one verification path covers all of them
breach:
  condition: a compression oracle needs attacker-influenced input reflected into the same compressed response as the secret, and responses are compressed here per decision:streaming-response-compression
  how_common: a search page echoing its query, an error quoting input, or a form redisplaying what was submitted all satisfy it, so this is ordinary rather than exotic
  closed_by: the per-response pad, which is why Rails and Django both do this
  cost: a few lines of derivation and no change to the client, the wire format, or the module
pre_session_forms:
  the_gap: a session-held secret exists only where a session does, so an unsafe form served to a visitor without one has nothing to carry
  the_real_case: an application's own unsafe form on a public page, such as a contact or search post
  not_the_login_flow: the shipped authentication mode is OIDC authorization code with PKCE, so the credential form belongs to the provider and the callback is protected by its own state parameter rather than by this token
  resolved_by: decision:anonymous-csrf-secret-storage, which carries the anonymous secret in a signed cookie and writes nothing
  why_signed_and_not_encrypted:
    requirement_is_integrity: the value must be unforgeable, not unreadable, since a random secret identifies nothing and hiding it from the browser holding it buys nothing
    encryption_is_self_defeating: the runtime has to read the cookie to put the token on a header, so any encryption would need a key the browser also holds
  cost: one signing key and no storage, so the anonymous population is free to serve regardless of size
acceptance:
  - a token exists wherever a secret does: for every session, and for an anonymous visitor only where that path is enabled
  - logging out clears the cookie and no later request presents a usable token
  - a login rotation replaces the token and the same response carries the new cookie
  - a form, a runtime request, and a rendered page in one session all verify against the one secret
  - two renders of one page emit different token bytes, so a compression oracle accumulates nothing
  - a token whose length or encoding is wrong is refused as a mismatch rather than crashing the check
  - the token never appears in a URL, a query string, a log line, or template scope
  - a page held open across a rotation still posts successfully, because the runtime reads the cookie at request time
  - a render with no session and no unsafe form succeeds; one with an unsafe form fails unless the anonymous path is enabled, rather than emitting an empty field
  - the secret stays out of the session request view, per data:session-record, and out of every diagnostic
```
