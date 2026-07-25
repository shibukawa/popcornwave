---
id: requirement:classic-rendering
type: requirement
title: Classic Rendering
---
Generated typed templates produce complete responses and reusable presentational components from already-loaded data.

```yaml
features:
  - layouts and nested presentational components
  - contextual automatic escaping via policy:template-escaping
  - explicit narrow raw-output escape hatch
  - status and headers selected before body write
  - HTML, redirect, download, JSON, XML, CSV, and declared output modes
constraint: presentational components do not fetch data
implementation: requirement:contrib-html-template
```
