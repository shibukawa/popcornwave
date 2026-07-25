---
id: decision:root-pw-api
type: decision
title: Root pw API
---
Popcorn Wave hides routine TinyBind usage behind the root pw package while preserving standard net/http handler signatures and intentional low-level escape hatches.

```yaml
status: accepted
preserve_in_handler:
  - "pw.Parse[Request](r)"
  - "pw.WriteHTML(w, r, template, params)"
  - "pw.WriteAPI(w, r, response)"
  - "pw.WriteProblem(w, r, err)"
  - "pw.NewStream[Event](w, r)"
rationale:
  - system:tinybind remains the generator and runtime implementation
  - the small root vocabulary keeps generated analysis aligned with runtime behavior
  - standard handlers remain independently testable
boundaries:
  - no custom handler invocation abstraction
  - no reflection-based automatic handler invocation
  - low-level TinyBind packages remain importable by deliberate application choice
```
