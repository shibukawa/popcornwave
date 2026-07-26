---
id: policy:devidp-safety
type: policy
title: Development IdP Safety Policy
---
requirement:contrib-devidp authenticates nobody, so it must be impossible to start it in a deployed environment, reach it from a remote network, or ship it inside an application binary.

```yaml
environment:
  - refuse startup when data:runtime-environment resolves to prod or production
  - log a development-only warning containing the issuer at every startup
network:
  - bind loopback only, with no opt-out, because nothing else bounds who may sign in
  - reject a non-loopback listen address before the listener opens
  - HTTP is allowed because the issuer is a loopback development URL under policy:oidc-security
  - CORS is allowed only for configured client origins
  - outbound requests are never made; the provider is a server only
ephemeral_clients:
  - only api:cli-dev and api:testutil-idp register ephemeral clients, and only while they own the provider process
  - credentials come from crypto/rand per run and are never written to disk or to a shell the developer keeps
  - an ephemeral client accepts a redirect URI only when the host is loopback, matching RFC 8252 section 7.3 port-agnostic loopback handling
  - the accepted redirect URI is logged at each authorization so an unexpected callback target stays visible
  - clients declared in data:devidp-config keep exact redirect URI matching with no relaxation
  - the post-logout redirect is exempt from registration for every client, but must stay local, and each acceptance is logged
  - injected credentials are masked wherever configuration provenance is logged
keys_and_tokens:
  - the signing key is generated per process from crypto/rand unless an explicit development key path is configured
  - a configured key path is a development fixture and is never written into generated project output
  - token, code, and key material are held in memory only and are destroyed by Provider.Close
  - token lifetime has a hard upper bound regardless of configuration
  - never log codes, verifiers, access tokens, ID Tokens, or client secrets
build:
  - api:cli-build fails when the application under build imports contrib/devidp
  - api:cli-init never scaffolds a contrib/devidp import into application source
  - devidp.toml is development tooling input and is excluded from release artifacts under policy:release-integrity
client_expectations:
  - requirement:contrib-oidc applies its normal verification; no relying-party check may be relaxed for this provider
  - policy:oidc-admission still governs whether a verified development identity may enter the application
  - the provider never issues tokens for a subject absent from data:devidp-config
non_goals:
  - production or staging identity
  - user directory, registration, or credential storage
  - any authentication factor
```
