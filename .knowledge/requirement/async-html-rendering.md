---
id: requirement:async-html-rendering
type: requirement
title: Async HTML Rendering
---
A handler adopts progressive HTML rendering by supplying async parameter values only; no other application change is required.

```yaml
dependency: system:tinybind v0.1.20 htmlbind async await boundaries
handler_delta:
  required:
    - build async values with api:async-html-value
    - pass them through the generated Params struct
  unchanged:
    - api:html-response WriteHTML call site and signature
    - handler registration in flow:handler-registration
    - decision:implicit-document-shell document resolution
    - error return style and api:problem-response mapping
    - no handwritten import of system:tinybind
template_delta:
  - declare `async T` on a component parameter or record field
  - read it only inside an await clause with a required fallback
  - optional recover clause renders a safe error subtree
selection: decision:automatic-async-render-selection
bounds: policy:async-render-bounds
wire: api:html-boundary-protocol
client_runtime: requirement:external-boundary-runtime
configuration: data:html-render-config
criteria:
  - a chain with no await block keeps the current buffered response byte for byte
  - a chain that can open a boundary streams shell and fallbacks before slow work settles
  - two boundaries in one response resolve concurrently, not serially
  - one async value shared by a wrapper and the leaf runs its work once
  - a required async value left unset fails before commit as a 500 api:problem-response
  - an optional async value left unset renders as absent and opens no boundary
  - a browser without the boundary runtime keeps every committed fallback and stays readable
  - a boundary failure with a recover clause renders it and never changes a committed status
  - a boundary failure without one follows decision:unhandled-boundary-escalation rather than leaving a stuck fallback
  - flow:template-generation rejects an async read outside an await clause
example:
  project: examples/async_render
  index: the entry page links one route per behaviour, so each path is reachable on purpose rather than by luck
  routes:
    success: one settled parameter beside two pending ones
    fail_with_catch: the same page with a failing dependency whose boundary declares recover, contained locally
    fail_without_catch: a failing dependency whose boundary declares none, escalating per decision:unhandled-boundary-escalation
non_goals:
  - handler-visible streaming API
  - flow:partial-refresh boundary reuse across a document lifetime
  - document-lifetime unique boundary identifiers
  - head contribution from a Fragment passed through a Params field
```
