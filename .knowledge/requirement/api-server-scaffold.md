---
id: requirement:api-server-scaffold
type: requirement
title: API Server Preset Scaffold
---
The api-server preset of requirement:init-presets writes a resource server that verifies a bearer token on the first command, against a development issuer nobody runs, and refuses every path to production until an operator names a real one.

```yaml
audience: actor:application-developer building a machine-facing API
mode: requirement:jwt-only-api-authentication, offered here per decision:jwt-only-preset-scaffolding
project_shape:
  router: registered only, so the scaffold is the handlers tree, its route example, and the generated OpenAPI document
  no_page_tree: an API answers no navigation, so nothing under concept:page-tree is written
  no_landing_page: the handlers tree carries the bearer route and no home.pw.html, because every caller arrives with an Authorization header and none of them renders a page
  document_shell_stays:
    written: templates/document.pw.html and the .pw.html error templates, unchanged from the browser presets
    why: api:error-renderer still answers a browser that reaches a failing route, and requirement:typed-http-contract answers every machine client through api:problem-response; dropping the shell would leave the first case with nothing to render
    revised: this reverses the earlier statement here that neither is written, which was specified before the scaffold was built and did not survive contact with the error renderer
  no_tailwind: nothing serves CSS
configuration:
  written_everywhere:
    mode: auth.mode jwt_only
    admission: auth.jwt.admission authenticated, per policy:bearer-admission, with the comment that names what it admits and the claim mode beside it
    identity_claim: sub, the data:external-identity default stated rather than inherited
    max_token_lifetime: a stated value, because requirement:jwt-only-api-authentication refuses to start without one
    revocation: disabled, which is a stated decision rather than an omission
    algorithms: RS256 only, because the verification key is a published JWKS entry and a symmetric algorithm would let anyone holding it mint tokens
  cross_origin:
    written: a commented security.cors block in the base configuration, naming enabled, allowed_origins and allowed_methods
    inert: enabled defaults false, so an uncommented block is the only way to turn it on
    why_this_preset: requirement:cors-middleware names this scaffold's reader as its driving case, a browser page on another origin calling this API with a bearer token, and the block is where that reader looks
    not_the_openapi_document: the generated document answers a wildcard origin on its own, per policy:operational-endpoints, so nothing about it belongs in the block
  development_placeholders:
    fields: auth.jwt.issuer, auth.jwt.audience, and auth.jwt.allow_loopback_http
    values: a loopback issuer on a port nothing runs on, the project name as the audience, and loopback http allowed for the first of those
    superseded: leaving the issuer and the audience empty for the operator to supply, on the reasoning that a value which parses is a value somebody ships
    why_that_failed:
      measured: the mode validates the whole auth.jwt prefix at startup with no development exemption, so an empty issuer refuses to start under pw dev too
      consequence: the empty version produced a project that could not run at all until an authorization server existed, which is the state policy:dev-token-relaxation exists to prevent
      second_reason: the account a request resolves to is derived from the issuer and the subject together, so a token has to name an issuer before admission can reach an account at all
    what_keeps_it_safe:
      scope: config.dev.toml only; no other environment file is scaffolded, and each one supplies its own
      inert: the relaxed path never calls the verifier, so nothing fetches metadata or a key set from the placeholder
      shape: a loopback address on an unused port, which is the form least mistakable for a provider somebody should point at
      unchanged: the framework still applies no default, so an absent key fails startup naming the field
    replacing_them: AUTH_JWT_ISSUER and AUTH_JWT_AUDIENCE, named in the scaffolded comment
  config_dev_only:
    field: auth.jwt.dev.trust_unverified_tokens true
    guarded_by: policy:dev-token-relaxation, whose build, environment, configuration, and loopback locks all still apply
    effect: pw dev serves a hand-written token from loopback, so the project is developable before an authorization server exists
    never_elsewhere: the field is absent from every other environment file, and rule:production-readiness-checks reports it as an error if one carries it
    supersedes: the policy:dev-token-relaxation rule that api:cli-init never scaffolds the field
first_run:
  works: pw dev, then the curl the scaffolded comment spells out, reaching the handler as data:request-authentication
  token_shape: iss and sub both present; nothing checks what the issuer says, and the pair is what the account is derived from, which is why the comment shows both rather than a subject alone
  fails_loudly:
    without_the_tag: a binary built without pwdev refuses on the development field rather than ignoring it
    outside_dev: the same configuration under stg or prod refuses naming that field
  next_steps_notice: the two fields to replace, and that revocation and its migration are a later api:cli-add away
storage:
  database: none
  why_it_can_be_none: plugin/auth requires middleware.rdb under jwt_only for the registered admission allowlist or the revocation list, and this scaffold takes neither; the authenticated admission mode and revocation off are the pair that keeps the project relational-free
  consequence: choosing the registered admission mode or enabling revocation later is what adds the database, and requirement:preset-customization-docs says so where the reader decides it
  alternative_not_taken: DynamoDB, which requirement:dynamodb-auth-backend now makes viable for a browser login; jwt_only stores nothing it could hold, since the token carries the identity and no ceremony creates an account
  session: cookie, the backend that stores nothing, because the mode takes no session for authentication and provisioning storage nothing reads would be a table to explain
  enabling_revocation: pw add database, then the auth.jwt revocation keys and the popcornweb_revoked_token migration, which requirement:preset-customization-docs owns
starter_handler:
  shows: one registered route reading data:request-authentication and answering a typed response
  does_not_show: an authorization decision, which policy:bearer-admission leaves to the application and which a scaffold cannot guess
  openapi: the generated document, which is the reason an API project takes the registered router
linking:
  rule: the scaffold writes a blank import of plugin/auth
  why: the bearer mode registers no account seam, so nothing else in the project reaches the package, and an unlinked package registers no extension
  what_it_looked_like_without_it: the [auth] section loaded as plain keys nobody validated, so startup accepted anything and every request arrived unauthenticated while the configuration read as though it were verifying tokens
  found_by: running the scaffolded project and curling it, which no test of the written files could have caught
acceptance:
  - a project created from this preset admits the token its own scaffolded comment shows how to build, with no edit, and answers the verified subject
  - a binary built without the development tag refuses to start on auth.jwt.dev.trust_unverified_tokens rather than ignoring it
  - the development configuration carried into a stg or prod environment refuses to start naming that field
  - the scaffolded main links plugin/auth, so the [auth] section is read by something
  - the authentication question and --auth still refuse jwt_only, per decision:jwt-only-preset-scaffolding
non_goals:
  - issuing a token, which requirement:contrib-devidp does not do in this form
  - scaffolding revocation, its table, or its store
  - a browser login beside the token, which the one-mode rule of data:authentication-runtime-config forbids
```
