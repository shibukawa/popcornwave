---
id: decision:dev-console-consolidation
type: decision
title: One Development Console Listener
---
Every api:cli-dev web surface is mounted on one loopback listener behind one index, rather than one listener per tool, and requirement:dev-telemetry-viewer becomes the first pane rather than the host.

```yaml
status: accepted
context:
  - decision:dev-telemetry-viewer-adoption already put the mount point in pw hands so that a later console could host the viewer beside its own surfaces
  - requirement:template-storybook, requirement:dev-asset-inspector, and requirement:dev-api-reference are three more surfaces arriving at once
  - api:cli-dev already prints a growing set of URLs for the application, the identity provider, and the viewer
decision:
  listener: one, serving the index and every pane
  ownership: pw owns the mux; the viewer is mounted into it rather than mounting pw into the viewer
  reversal: requirement:dev-telemetry-viewer no longer owns the listener it introduced
port:
  key: dev.console.port
  default: fixed rather than the reserved 0 the viewer shipped with
  reason: a reserved port moves every run, and a surface the developer returns to all day cannot be bookmarked; the same argument already moved dev.idp.port to a scaffolded value
  concurrency_cost: two projects running pw dev at once collide, which a reserved port avoided; the fix is editing the key, and the collision is reported by address
  external_receiver: dev.otel.port is retained for the OTLP receiver alone, because an external exporter may have to find it at a number agreed in advance
alternatives_rejected:
  listener_per_tool:
    why_not: the startup report becomes a list of numbers with no relation between them, and every tool repeats bind, collision, and lifetime handling
  mount_under_the_application:
    shape: serve the console from the application process on an application route
    why_not: it puts development surfaces in the artifact api:cli-build produces and makes the console die with the application it exists to report on
  keep_the_viewer_as_the_host:
    why_not: the upstream listener answers to system:localotelviewer, so every pw pane would live at the sufferance of a dependency's routing
sequencing: the viewer moves onto the console listener before the first new pane lands, so no pane is written against a mount point that is about to change
```
