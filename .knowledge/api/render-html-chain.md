---
id: api:render-html-chain
type: api
title: TinyBind HTML Render Chain
---
TinyBind v0.1.15 composes generated document and layout wrappers around one generated page fragment.

```yaml
owner: system:tinybind htmlbind package
signature: "htmlbind.RenderChain(io.Writer, []htmlbind.Wrapper, htmlbind.Fragment) error"
order:
  wrappers: outermost first
  leaf: innermost page fragment
generated_values:
  fragment: "<Name>(<Name>Params) htmlbind.Fragment"
  wrapper: "Bind<Name>(<Name>Params) htmlbind.Wrapper for a component with an unnamed slot"
behavior:
  - validate the leaf and wrappers before writing
  - merge head contributions in composition order
  - fill each wrapper unnamed slot with the next wrapper or leaf
  - render an empty wrapper list like htmlbind.Render
compatibility: v0.1.15 generated Fragment and Wrapper APIs replace earlier direct-writer template APIs and require regenerated call sites
```
