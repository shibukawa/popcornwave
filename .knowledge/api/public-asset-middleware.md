---
id: api:public-asset-middleware
type: api
title: Public Asset Middleware
---
Application lifecycle construction selects one static-file middleware implementation behind the same public asset registration.

```yaml
registration:
  implementation: github.com/shibukawa/popcornwave/middlewares
  generated_project: public.go init calls middlewares.RegisterPublicFS(PublicFS())
  compatibility_override: WithPublicFS(fs.FS)
  linker: generated main bootstrap blank-imports the public package
  lifecycle: api:application-lifecycle
  mount: data:server-runtime-config server.public.mount
selection:
  development:
    condition: api:cli-dev reserved pwdev build mode
    implementation: decision:development-public-assets
  production:
    condition: ordinary api:cli-build or application build
    lookup: policy:public-asset-resolution
    encoding: policy:public-asset-negotiation
response:
  found: 200 with inferred Content-Type and selected representation
  revision_stamped: long max-age with immutable under a requirement:framework-script-assets revision segment
  mount_without_trailing_slash: 308 to the canonical mount root
  missing_or_hidden: 404
  unsupported_method: 405 with Allow
middleware_order:
  - public assets bypass session, CSRF, and policy:authenticated-path-protection
  - security response headers, access logging, tracing, and request recovery still apply
  - dynamic response compression skips this middleware response
startup:
  - server.public.enabled requires a registered or explicitly supplied fs.FS in production
  - reject an invalid mount before accepting requests
  - reject application content under the reserved requirement:framework-script-assets subtree
```
