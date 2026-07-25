---
id: decision:development-public-assets
type: decision
title: Development Public Assets
---
`pw dev` serves the project public directory directly so asset edits require neither precompression nor application rebuild.

```yaml
source: project-root public/
mode:
  selection: reserved pwdev build mode used by api:cli-dev
  enabled: server.public.enabled
  mount: server.public.mount
  local_read: forced true independently of server.public.read_local
  embedded_fallback: disabled
  content_encoding: identity only
behavior:
  - do not run flow:public-asset-build
  - do not create, update, delete, inspect, or serve .zstd sidecars
  - do not negotiate Accept-Encoding or emit Content-Encoding
  - do not rebuild or restart Go for public file changes
  - open each eligible file from the local filesystem per request
middleware:
  - selected while api:application-lifecycle constructs api:public-asset-middleware
  - shares mount, method, path-security, and middleware-order behavior with production
rules:
  - path and file security remain policy:public-asset-resolution
  - missing local files return 404 even when an older embedded file exists
  - production behavior remains requirement:public-asset-delivery
rationale: direct reads provide immediate frontend feedback and avoid compression work in the development loop
```
