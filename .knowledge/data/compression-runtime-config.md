---
id: data:compression-runtime-config
type: data
title: Compression Runtime Config
---
The `compression` binding controls HTTP response compression through requirement:contrib-zstd.

```yaml
fields:
  zstd_enabled: bool
  minimum_size: bytes
  content_types: string list
  etag: bool
rules:
  - honor Accept-Encoding negotiation
  - emit Vary for negotiated encoding
  - skip pre-compressed, streaming, no-transform, and ineligible response types
  - validate minimum size and content-type patterns at startup
```
