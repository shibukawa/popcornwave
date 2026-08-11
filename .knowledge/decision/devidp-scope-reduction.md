---
id: decision:devidp-scope-reduction
type: decision
title: Development IdP Scope Reduction
---
Popcorn Wave reimplements the smallest useful subset of system:oidcld as requirement:contrib-devidp instead of depending on it, because the framework needs an in-repo OIDC counterparty rather than a local edge platform.

```yaml
status: proposed
keep:
  - password-free user selection login
  - declarative user roster with extra claims and extra scopes
  - Authorization Code with S256 PKCE
  - device authorization for browserless TinyGo client development
  - discovery, JWKS, and RS256 ID Tokens
  - UserInfo
drop:
  - local certificate authority and TLS termination, because decision:local-tls-proxy-boundary already owns local TLS
  - reverse proxy, static hosting, and OpenAPI mocks, because api:cli-dev already runs the local service set
  - client credentials grant, because no application consumer requires machine identity
  - EntraID v1 and v2 compatibility modes, because no framework consumer targets MSAL
  - refresh tokens and RP-initiated logout, because requirement:contrib-oauth defers refresh scheduling
  - developer console and MCP surfaces, because they are operator tools rather than protocol surfaces
naming:
  package: contrib/devidp
  reason: the dev-only boundary stays visible at every import site and in build failures
placement:
  decision: contrib package with api:cli-dev and api:testutil-idp consumers
  rejected:
    - separate pw subcommand, because no consumer outside the framework loop was requested
    - testutil-only helper, because api:cli-dev needs the same provider
compatibility_exemption:
  rule: policy:contrib-compatibility TinyGo matrix does not apply to contrib/devidp
  reason: it is a host-side development tool under decision:host-tools-target-runtime and never links into an application binary
  retained: bounded input, no reflection-driven discovery, context-aware shutdown, host Go tests
tool_owned_wiring:
  decision: api:cli-dev and api:testutil-idp own client registration and inject issuer and credentials into the application
  reason: a development client secret carries no security value, so making the developer copy one between two files is pure friction
  effect:
    - data:devidp-config clients are optional and exist only for manually driven or external clients
    - the application declares its callback path and nothing else about the provider
    - rotating credentials every run removes any stale-secret failure mode
  boundary: injection is limited to the process the tool starts, and never writes secrets to disk or to the developer shell
configuration_format:
  choice: TOML
  reason: every other Popcorn Wave input is TOML through data:project-config and policy:config-file-resolution
value:
  - requirement:contrib-oidc and requirement:contrib-oauth gain a controllable counterparty for requirement:contrib-acceptance fixtures
  - flow:oidc-account-login becomes runnable in api:cli-dev and api:test-run without external IdP credentials
scope_boundary: policy:devidp-safety forbids production use, so no account, consent, or credential feature is added later without a new decision
```
