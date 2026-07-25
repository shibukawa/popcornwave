---
id: system:tinygodriver
type: system
title: tinygodriver
---
tinygodriver is the external owner of reusable TinyGo compatibility packages consumed by Popcorn Wave.

```yaml
module: github.com/shibukawa/tinygodriver
source: https://github.com/shibukawa/tinygodriver
packages:
  netdev: github.com/shibukawa/tinygodriver/netdev
  httpmux: github.com/shibukawa/tinygodriver/httpmux
  httprevproxy: github.com/shibukawa/tinygodriver/httprevproxy
  zstd: github.com/shibukawa/tinygodriver/compress/zstd
  sqlite: github.com/shibukawa/tinygodriver/database/sqlite
  cgosqlite: github.com/shibukawa/tinygodriver/database/cgosqlite
roles:
  netdev: host TCP/IP Netdever registration for TinyGo
  httpmux: Go 1.22+ ServeMux-compatible routing for TinyGo
  httprevproxy: TinyGo-compatible net/http/httputil.ReverseProxy subset
  zstd: bounded TinyGo encoder with optimized host fallback
  sqlite: portable database/sql SQLite facade
  cgosqlite: native SQLite driver used by TinyGo
standard_go:
  netdev: no-op registration
  httpmux: alias of net/http.ServeMux
  zstd: optimized github.com/klauspost/compress backend
  sqlite: host-selected database/sql driver
```
