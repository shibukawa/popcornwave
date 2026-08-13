---
id: policy:csrf-protection
type: policy
title: CSRF Protection
---
CSRF middleware validates a session-bound synchronizer token and request origin on configured unsafe browser requests.

```yaml
scope:
  unsafe_methods: every method except GET, HEAD, and OPTIONS
  protected: canonical path matches security.csrf.include and no security.csrf.exclude
  page_actions:
    requirement: the api:page-action-endpoint prefix must be covered, because those are POST endpoints reachable with ambient credentials and system:tinybind wires no protection around them
    covered: the middleware exists and api:cli-init writes the prefix into the include list of a page-tree project
  ordering:
    where: a framework extension below the session slot, so the secret it compares against has been resolved, and below the authentication slot, so a plugin's own login and callback endpoints answer above it rather than needing a configured exclusion
    why_not_the_outer_stack: the framework middleware stack wraps the extension chain from outside, where no session has been resolved and every unsafe request would be refused
  pattern_grammar: one shared implementation with policy:authenticated-path-protection rather than a second one that can drift
  patterns: same segment grammar and exclude precedence as policy:authenticated-path-protection
  form_insertion: system:tinybind writes the hidden field into every unsafe form itself from v0.3.3, so coverage no longer depends on an author remembering one; requirement:module-native-csrf carries what that changed
  cache_exclusion: a component holding an unsafe form cannot be output-cached, enforced at generation, because a stored body would hand one session's token to the next visitor
  update_endpoints:
    action_response: requirement:action-response-update mutates and carries ambient credentials, so it is protected like any other unsafe request; the transport is the header, because the runtime issues the fetch
    navigation_and_redraw: requirement:navigation-delta-rendering and requirement:reloadable-component-endpoint are GETs that must stay side-effect free, so no token gates them; origin defence still applies to a credentialed read
    live: the delivery stream of api:live-delivery-protocol is a GET carrying a custom header, which a cross-origin form or link cannot set
token:
  secret: one registered session.Private slot for every visitor signed in or not, per decision:csrf-secret-as-a-session-slot, riding a sealed cookie while the session is anonymous and moving onto the configured backend at login; minted and destroyed with the session per requirement:csrf-token-lifecycle
  request_value: a per-response value derived from the secret with a fresh pad, so a compression oracle sees different bytes every time
  masking_retained: an earlier revision dropped it, reasoning that the module compares by equality with no unmasking step; that was wrong, because the compared value is this framework's to compute and is recomputed from the incoming token's own pad, per requirement:csrf-token-lifecycle
  sources:
    - configured form field, written into every unsafe form by system:tinybind itself per requirement:module-native-csrf, so no author has to remember it
    - configured HTTP header, attached by requirement:unified-update-runtime to every mutating update request
  transport_to_the_browser: a cookie the session manager writes beside the session cookie and the runtime reads when it issues a request, so a rotation reaches an already-open page; never inline script and never a generated JavaScript literal
  cookie_is_the_one_readable_by_script: policy:cookie-value-protection keeps every other cookie away from script, and this is the documented exception, because a runtime that cannot read the token cannot send it
  why_not_the_script_tag: the module's default puts the token in the runtime configuration written once at render, which cannot be refreshed; requirement:unified-update-runtime is this framework's own asset and deviates
  cookie_is_not_the_counterpart: verification compares against the session's own secret, so the cookie is transport and the forbidden naive double-submit shape is not what this is
  three_channels_one_secret: the cookie, the header, and the hidden field carry different derived bytes that all unmask to one secret, so one verification path covers every channel
  forbidden:
    - URL query
    - request logs
    - error details
    - Web Storage, since a token there outlives the session that bound it
validation:
  - require a secret on a protected unsafe request; it is issued to an anonymous visitor as readily as to a signed-in one, so this is a requirement on the session mechanism being enabled rather than on the visitor having authenticated
  - require constant-time token validation against a value recomputed from the incoming token, never against the stored secret directly
  - require same-origin Origin or trusted exact origin; use strict Referer fallback only when Origin is absent
  - compare a whole origin, scheme included, through the one implementation named under origin_check
  - reject missing, multiple, malformed, expired-session, or mismatched tokens
  - refuse a request whose path cannot be matched unambiguously, since it could select a different routed target than the one the check decided about
  - the response never says which check failed; the reason reaches the log only, because naming it tells a caller which half to work on
  - return HTTP 403 through api:error-renderer without calling the application handler
origin_check:
  owner: the middlewares CSRF implementation, which reconstructs this request's origin as scheme and host, refuses a null Origin, requires a scheme on the Referer fallback, and consults the configured trusted origins
  forwarded_headers:
    for_this_comparison: still not trusted; a deployment behind a scheme-rewriting proxy names its origin in TrustedOrigins rather than having the check accept a value a caller can assert, because a declared origin is an answer the deployment already owed
    for_the_self_origin: decision:forwarded-header-trust governs the scheme this request reconstructs for itself, which is a different question and has no declared answer; requirement:proxied-request-identity is why the two are now stated apart
  single_implementation: internal/requestorigin, shared with the plugin/auth login, logout, passkey, and presence endpoints; the same reason pattern_grammar gives, applied to origin comparison, because a second copy is a second set of rules that drifts from this one
  auth_trusted_origins: the passkey origin allowlist and the origin of the OIDC redirect URL, both of which the deployment already had to declare for another reason, so a TLS-terminating proxy needs no new setting and no header is inferred
lifecycle:
  - create secret with the session, which for the shipped authentication mode means at login
  - rotate with session creation, login rotation, and privilege changes, writing the new cookie on the same response
  - remove with session revocation, clearing the cookie on the same response
  - detail: requirement:csrf-token-lifecycle
  - anonymous_is_covered: decision:csrf-secret-as-a-session-slot issues the secret to every visitor through one session.Private slot, which rides a sealed cookie while the session is anonymous, so a public page's unsafe form carries a token and no server row is written for a crawler that merely loaded it
  - corrected: 2026-08-12; an earlier line here called this unresolved and said issuing a session on demand was a capability this framework did not have, which the slot model had already answered
  - what_is_actually_required: the session mechanism being enabled, not a visitor being signed in; pwsession.Setup returns no manager when session.enabled is false, and without one there is no slot to hold a secret
rules:
  - SameSite cookies supplement but do not replace token validation
  - login, OIDC callback, bootstrap, webhook, and non-browser API exclusions are explicit configuration decisions
  - bearer-only requests without cookie authority remain application policy
  - the token is supplied to a render as an option read from api:request-context-accessors, never as a component parameter and never through template scope
  - a render with no session states that explicitly, so a forgotten token is distinguishable from a deliberate absence
  - an unsafe form with no token supplied fails the render rather than emitting an empty field
  - a token never enters a cached component body or a reusable layout frame, since a validator computed over that output would freeze it
```
