---
id: system:tinygodriver
type: system
title: tinygodriver
---
tinygodriver is the external owner of reusable TinyGo host networking and HTTP compatibility packages consumed by Petitweb.

```yaml
module: github.com/shibukawa/tinygodriver
source: https://github.com/shibukawa/tinygodriver
packages:
  netdev: github.com/shibukawa/tinygodriver/netdev
  httpmux: github.com/shibukawa/tinygodriver/httpmux
  httprevproxy: github.com/shibukawa/tinygodriver/httprevproxy
roles:
  netdev: host TCP/IP Netdever registration for TinyGo
  httpmux: Go 1.22+ ServeMux-compatible routing for TinyGo
  httprevproxy: TinyGo-compatible net/http/httputil.ReverseProxy subset
standard_go:
  netdev: no-op registration
  httpmux: alias of net/http.ServeMux
```
