---
id: requirement:public-asset-delivery
type: requirement
title: Public Asset Delivery
---
The conventional project-root public directory becomes an embedded, optionally overlaid, externally reachable static asset tree.

```yaml
source: project-root public/
scaffolded_embed: project-root public.go
build: flow:public-asset-build
runtime: api:public-asset-middleware
configuration: data:server-runtime-config
development: decision:development-public-assets
rules:
  - public is the only framework-owned static directory convention
  - serving precompressed sidecars performs no runtime compression or decompression
  - ordinary application routes remain responsible for non-public content
  - application startup rejects mount collisions
  - disabling the endpoint registers no public route but does not change the built binary
references:
  - https://pkg.go.dev/embed
```
