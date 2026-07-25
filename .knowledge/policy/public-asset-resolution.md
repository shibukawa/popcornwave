---
id: policy:public-asset-resolution
type: policy
title: Public Asset Resolution
---
Public asset lookup preserves source consistency while allowing an explicit working-directory override of embedded assets.

```yaml
production:
  source_order:
    - when server.public.read_local is true and ./public contains the original regular file, select the local layer
    - otherwise select the embedded PublicFS layer
development:
  source: decision:development-public-assets
path:
  - strip the normalized configured mount prefix
  - percent-decode once through net/http request semantics
  - clean as a slash-separated relative fs.ValidPath
  - reject empty ambiguity, dot segments, backslashes, NUL, and traversal
representation:
  - inspect only the selected layer
  - choose {path}.zstd when policy:public-asset-negotiation allows it and the matching sidecar exists
  - otherwise choose the original file
security:
  - reject a symbolic-link local root, symbolic links below it, and non-regular local files
  - never fall back to an embedded sidecar for a local original
  - never expose directory listings, dot-prefixed path segments, or .zstd URLs
  - resolve a directory only as its index.html when present
```
