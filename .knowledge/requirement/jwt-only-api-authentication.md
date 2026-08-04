---
id: requirement:jwt-only-api-authentication
type: requirement
title: JWT-Only API Authentication
---
The jwt_only mode of data:authentication-runtime-config makes an application a resource server: every request proves itself with a bearer JWT that somebody else issued, and the framework mounts no login, sets no cookie, and starts no ceremony.

```yaml
audience: actor:application-developer building a machine-facing API rather than a browser application
mode: auth.mode jwt_only, beside oidc_only, oidc_passkey, and passkey_only
availability: decision:jwt-only-mode-not-scaffolded; the binding accepts the mode, and no command offers it
role:
  is: a resource server that verifies a token
  is_not: an authorization server that issues one
scope:
  verification: policy:access-token-verification over requirement:contrib-jwt
  key_discovery: issuer metadata and JWKS, reusing the retrieval and refresh behavior of requirement:contrib-oidc
  admission: policy:bearer-admission, which is where a deployment says only this organization enters
  revocation: policy:token-revocation over a token identifier the deployment can list
  development: policy:dev-token-relaxation, which admits a hand-written token under `pw dev` so the mode can be developed against with no authorization server running
  result: data:request-authentication with method bearer
  guard: policy:authenticated-path-protection unchanged, with the unauthenticated answer forced to unauthorized
surface: api:bearer-authentication
flow: flow:bearer-request-authentication
what_it_replaces:
  endpoints: none; api:authentication-endpoints mounts nothing in this mode, because login, callback, and logout are all redirects a machine client cannot follow
  session: none for authentication; concept:session-storage-boundary still holds, and an application may register its own slots, but data:request-authentication comes from the token rather than from a slot
  csrf: policy:csrf-protection does not apply to a request whose authority is a header the browser never attaches on its own
  bootstrap: decision:authentication-bootstrap-strategy has no meaning here, because no account is created by a ceremony this framework runs
account_link:
  claim: auth.jwt.identity_claim, defaulting to sub, with the stability contract of data:external-identity
  resolver: auth.SetAccountResolver, the same seam api:authentication-endpoints uses, so a handler reads auth.User the same way
  optional: a deployment doing pure claim-based authorization may run with no resolver, and auth.User then reports nothing while data:request-authentication still carries the verified subject and claims
assurance:
  freshness: the auth_time claim when the issuer sends one, and iat otherwise
  guard: api:assurance-guard reads that freshness, and both wrappers answer 401 in this mode
  header: the RFC 9470 insufficient_user_authentication challenge is emitted here, which is the Bearer-protected endpoint that concept reserved it for
  no_step_up: flow:step-up-reauthentication does not apply, because the framework did not authenticate the client and cannot send it anywhere; the 401 names the unmet max_age and the client re-authenticates with its own issuer
  strength: absent, for the reason api:assurance-guard already deferred it
required:
  - no permissive default anywhere in the auth.jwt prefix; the issuer, the audience, the admission rule, the maximum token lifetime, and whether revocation is enabled are all stated by the configuration or startup fails
  - verify before admitting, and admit before revoking, so a forged token never reaches an allowlist lookup or a store
  - refuse a request with no bearer credential rather than treating it as anonymous on a protected path
  - treat a token this framework cannot verify as absent rather than as invalid input, so a probe learns nothing
  - one issuer per application; a multi-issuer deployment is a non-goal below
  - validate the whole auth.jwt prefix at startup, and refuse every field the mode cannot honor, per the mode_validation rule of data:authentication-runtime-config
  - answer every refusal as api:problem-response, per requirement:typed-http-contract
acceptance:
  - a token signed by the discovered key, inside its window, carrying the configured audience is admitted and reaches the handler as data:request-authentication
  - a token whose signature, issuer, audience, expiry, or token type fails is refused with 401 and a stable category that names no claim value
  - a token omitting any of iss, sub, aud, exp, iat, or jti is refused, and an ID Token from the same issuer is refused on the token type check before anything else looks at it
  - a token whose exp minus iat exceeds auth.jwt.max_token_lifetime is refused, so a long-lived token cannot outlive a subject-form revocation entry
  - a configuration omitting the audience, the admission rule, the maximum token lifetime, or the revocation decision fails startup naming the field
  - a production build refuses to start on a configuration carrying auth.jwt.dev, and the same configuration under `pw dev` admits an unsigned token from loopback and refuses one from anywhere else
  - a token whose kid is unknown triggers at most one JWKS refresh, and a stream of unknown kids triggers no more
  - an admission rule that does not match refuses the request without creating or linking an account
  - a revoked token identifier is refused on the next request, and stays refused until its own expiry
  - a revocation store that cannot be reached refuses the request as an error rather than admitting it
  - the same handler code compiles and passes under jwt_only and under oidc_only, because both produce data:request-authentication
  - a configuration setting auth.protection.unauthenticated to redirect fails startup naming the mode
  - a configuration carrying an oidc or passkey field under jwt_only fails startup naming the field
non_goals:
  - issuing, refreshing, or exchanging a token; requirement:contrib-devidp remains development-only
  - opaque tokens and RFC 7662 introspection, which is a network call per request rather than a signature check
  - more than one issuer or more than one signing algorithm family in one application
  - mTLS, DPoP, and any other proof of possession; the bearer is the whole credential
  - authorization beyond admission, which stays with the application per data:request-authentication
  - a browser session established from a verified token, which would make this mode a login by the back door
  - mixing jwt_only with a cookie login in one application, per the one-mode rule of data:authentication-runtime-config
```
