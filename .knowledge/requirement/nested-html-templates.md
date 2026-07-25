---
id: requirement:nested-html-templates
type: requirement
title: Nested HTML Templates
---
Classic pages share one generated document shell by composing nested TinyBind templates at response time.

```yaml
dependency: system:tinybind v0.1.15 or later
document:
  source: templates/document.pw.html
  role: outermost wrapper
  contains:
    - doctype
    - html element
    - head element
    - body element
    - unnamed <slot /> inside body
page:
  source: each page .pw.html
  role: innermost leaf
classic_render:
  api: api:render-html-chain
  wrappers:
    - generated BindDocument value
  leaf: generated page Fragment
  extension: insert generated layout wrappers between document and page without changing either template
compatibility:
  breaking_change: TinyBind v0.1.15 generated HTML code is not source-compatible with earlier direct-writer output
  migration:
    - regenerate every .pw.html artifact with the pinned TinyBind version
    - update classic handler rendering call sites to Fragment, Wrapper, and api:render-html-chain
acceptance:
  - api:cli-init creates templates/document.pw.html
  - generated classic handlers render document and page through one chain
  - page templates contain page content without duplicating the document shell
  - another wrapper can be inserted between document and page
```
