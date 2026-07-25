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
defaults:
  health.path: /healthz
  readiness.path: /readyz
  openapi.path: /openapi.json
rules:
  - expose the port as server.port, --port, and PORT for container-oriented configuration
  - validate the port, positive limits, durations, and proxy networks at startup
  - graceful shutdown uses shutdown_timeout where supported
  - local TLS termination follows decision:local-tls-proxy-boundary
  - policy:operational-endpoints defines endpoint behavior and access
  - reject duplicate endpoint paths and collisions with application routes
  - OpenAPI documents are assembled from generated package fragments
```
