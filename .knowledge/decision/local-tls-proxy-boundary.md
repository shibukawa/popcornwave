---
id: decision:local-tls-proxy-boundary
type: decision
title: Local TLS Proxy Boundary
---
Popcorn Wave selects a local sidecar or egress proxy as the default TLS boundary for TinyGo services that connect to external systems.

```yaml
status: accepted
application_hop:
  endpoint: loopback or the same Pod network namespace
  transport: plaintext application protocol is allowed only inside this boundary
proxy_hop:
  transport: TLS required outside the local workload boundary
  responsibilities:
    - certificate-chain and hostname verification
    - SNI and protocol-specific TLS negotiation
    - upstream reconnect, health checking, and bounded retry
recommended:
  postgresql: pgBouncer with verified TLS upstream
  mysql: ProxySQL or another protocol-aware proxy with verified TLS upstream
  redis_valkey: stunnel, Envoy, HAProxy, or a managed service proxy
  http: an egress proxy that preserves the logical HTTPS origin
support_tiers:
  main: verified TLS upstream through the local proxy boundary
  experimental: direct TinyGo TLS when target-specific evidence exists
  unsupported: plaintext traffic across host, Pod, or workload trust boundaries
rationale:
  - TinyGo TLS and resolver APIs vary by target and remain incomplete for common maintained clients
  - a local proxy provides mature certificate validation and protocol negotiation without embedding a second TLS stack
  - application packages remain testable with bounded plaintext protocol fixtures on loopback
```
