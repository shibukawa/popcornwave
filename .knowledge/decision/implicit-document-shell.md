---
id: decision:implicit-document-shell
type: decision
title: Implicit Document Shell
---
Classic handlers render only a page fragment; the framework supplies the registered document wrapper.

```yaml
status: accepted
handwritten_surface: "pw.WriteHTML(w, r, Page(PageParams{...}))"
hidden:
  - templates/document.pw.html import and generated binder
  - pw.HTMLWrapper values
  - api:render-html-chain invocation
registration:
  source: templates/document.pw.html generated artifact
  lifecycle: package initialization registers one application document wrapper
  bootstrap: api:cli-init ensures the registration package is linked without handler references
runtime:
  - api:html-response resolves the registered document
  - call api:render-html-chain with document outermost and page as leaf
validation:
  - missing or duplicate default document registration is a startup error
  - handler code never selects or constructs the default document
layouts: future explicit layout selection may extend the registered chain without exposing the document shell
```
