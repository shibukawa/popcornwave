---
id: policy:outbound-transport-security
type: policy
title: Outbound Transport Security Policy
---
External traffic from TinyGo services crosses a verified TLS boundary even when the application speaks plaintext to a colocated proxy.

```yaml
boundary: decision:local-tls-proxy-boundary
scope:
  - external HTTP APIs and identity providers
  - telemetry exporters
  - PostgreSQL and MySQL
  - Redis and Valkey
local_hop:
  allowed_endpoints:
    - loopback
    - same Pod network namespace
  requirements:
    - bind the proxy listener so other workloads cannot reach it
    - retain protocol authentication when supported
    - never log credentials, tokens, session values, or connection URLs containing secrets
external_hop:
  requirements:
    - TLS with certificate-chain and hostname verification
    - SNI and upstream identity configured independently from the local listener
    - connection, response-size, retry, and shutdown bounds appropriate to the protocol
http:
  preferred: forward or egress proxy preserves the logical HTTPS URL and validates the upstream origin
  local_override: allowed only for an explicit same-workload listener with a separately pinned upstream identity
direct_tinygo_tls:
  status: supported for the SQL engines of decision:server-sql-support-tier, experimental elsewhere
  rule: direct TLS is not a prerequisite for first-class support when the proxy path passes acceptance tests
  sql_engines:
    mechanism: the driver upgrades the connected socket through the system:tinygodriver OS TLS backend
    bounds: decision:server-sql-support-tier platform_bounds, which a deployment must check before relying on it
    never: a platform without a backend refuses the connection instead of falling back to plaintext
forbidden:
  - plaintext across host, Pod, node, or workload trust boundaries
  - disabled certificate or hostname verification outside explicit loopback test fixtures
  - treating an unencrypted external network as private merely because it is internal
```
