---
id: vision:web-application-styles
type: vision
title: Classic and Modern Web Styles
---
Popcorn Wave supports classic handler-based web applications and an opt-in server-driven modern UI on one net/http foundation. Modern facilities extend rather than replace classic mode.

```yaml
principles:
  - standard Go and TinyGo targets
  - generated mapping instead of runtime reflection
  - net/http interoperability
  - server-rendered HTML with a small optional browser runtime
  - incremental adoption without unused modern runtime cost
layers:
  - requirement:shared-web-runtime
  - requirement:classic-web-acceptance
  - requirement:modern-web-acceptance
delivery: decision:web-runtime-delivery-order
architecture: decision:layered-web-runtime
```
