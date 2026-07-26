---
id: system:oidcld
type: system
title: OIDCLD
---
OIDCLD is an external local development identity provider and edge platform that requirement:contrib-devidp reduces to an embeddable subset.

```yaml
repository: https://github.com/shibukawa/oidcld
license: AGPL-3.0
role: reference implementation and prior art, not a dependency
shape:
  identity: OIDC and EntraID-compatible provider with password-free user selection
  configuration: YAML roster of users with extra claims and extra scopes
  extras:
    - managed local certificate authority and TLS termination
    - reverse proxy, static hosting, and OpenAPI mock responses
    - developer console and MCP surfaces
    - device authorization and client credentials grants
    - refresh tokens and RP-initiated logout
usage_rule: reimplement the required subset; do not vendor, fork, or link the upstream code
reduction: decision:devidp-scope-reduction
```
