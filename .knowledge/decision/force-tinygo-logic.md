---
id: decision:force-tinygo-logic
type: decision
title: Forced TinyGo Logic Build Tag
---
force_tinygo_logic selects TinyGo-compatible fallback implementations across dual-backend contrib packages during host Go builds.

```yaml
tag: force_tinygo_logic
scope: every contrib package with distinct host and TinyGo implementations
selection:
  host_default: "!tinygo && !force_tinygo_logic"
  tinygo_compatible: "tinygo || force_tinygo_logic"
rules:
  - one shared tag switches all applicable packages together
  - forced and default implementations preserve public APIs
  - host CI tests default and forced selections
```
