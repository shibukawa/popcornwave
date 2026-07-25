---
id: requirement:classic-web-acceptance
type: requirement
title: Classic Web Acceptance
---
A classic-only build remains a small practical TinyGo target with complete HTTP behavior and no modern UI dependency.

```yaml
criteria:
  - works without the Popcorn Wave browser runtime
  - uses standard requests and complete responses
  - interoperates with net/http handlers and middleware
  - supports typed binding, configuration, errors, templates, and OpenAPI
  - optionally supports requirement:tailwind-css-integration without a browser runtime
  - excludes component graph, patch protocol, and hydration dependencies
dependencies:
  - requirement:shared-web-runtime
  - concept:classic-web-style
```
