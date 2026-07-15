---
id: requirement:contrib-reverse-proxy
type: requirement
title: TinyGo Reverse Proxy
---
contrib/httprevproxy supplies a TinyGo-compatible ReverseProxy subset built only on supported net/http interfaces. Its distinct package name avoids confusion with the standard package while preserving source-level API migration.

```yaml
package: contrib/httprevproxy
public_api:
  - ReverseProxy implementing http.Handler
  - BufferPool
  - ProxyRequest with In and Out requests
  - ProxyRequest.SetURL
  - ProxyRequest.SetXForwarded
  - NewSingleHostReverseProxy
  - Rewrite callback
  - Transport http.RoundTripper
  - ModifyResponse callback
  - ErrorHandler callback
  - ErrorLog
  - FlushInterval
  - deprecated Director callback
required_behavior:
  - clone outbound request and preserve cancellation context
  - join target and inbound paths correctly
  - remove hop-by-hop request and response headers
  - remove untrusted Forwarded and X-Forwarded values before Rewrite
  - stream response bodies with bounded buffers
  - close request and response bodies on all paths
  - propagate status, end-to-end headers, and declared trailers
defaults:
  forwarded_headers: opt-in through SetXForwarded
  transport: http.DefaultTransport
deferred:
  - protocol upgrades and WebSocket tunneling
  - 1xx forwarding
  - HTTP/2-specific features
  - request and response dump helpers
compatibility: subset of net/http/httputil ReverseProxy; import path is intentionally different
evidence:
  tinygo_gap: https://tinygo.org/docs/reference/lang-support/stdlib/#nethttphttputil
  reference_api: https://pkg.go.dev/net/http/httputil#ReverseProxy
```
