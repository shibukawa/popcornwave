---
id: data:server-runtime-config
type: data
title: Server Runtime Config
---
The `server` binding controls HTTP listener lifecycle, limits, and proxy trust.

```yaml
fields:
  port: HTTP listen port
  read_header_timeout: duration
  read_timeout: duration
  write_timeout: duration
  idle_timeout: duration
  shutdown_timeout: duration
  max_request_body: bytes
  trusted_proxies: address or network list
  health.enabled: bool
  health.path: absolute path
  readiness.enabled: bool
  readiness.path: absolute path
  openapi.enabled: bool
  openapi.path: absolute path
  public.enabled: bool
  public.mount: absolute non-root path prefix
  public.read_local: bool
defaults:
  health.path: /healthz
  readiness.path: /readyz
  openapi.path: /openapi.json
  public.enabled: true
  public.mount: /public
  public.read_local: false
rules:
  - expose the port as server.port, --port, and PORT for container-oriented configuration
  - validate the port, positive limits, durations, and proxy networks at startup
  - graceful shutdown uses shutdown_timeout where supported
  - local TLS termination follows decision:local-tls-proxy-boundary
  - policy:operational-endpoints defines endpoint behavior and access
  - requirement:public-asset-delivery defines the public endpoint
  - local public root is ./public relative to the process working directory
  - public.mount is canonicalized to one leading and trailing slash and rejects root, dot segments, wildcards, queries, and fragments
  - reject duplicate endpoint paths, overlapping mounts, and collisions with application routes
  - OpenAPI documents are assembled from generated package fragments
```
