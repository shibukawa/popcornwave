---
id: policy:template-escaping
type: policy
title: Template Escaping Policy
---
Generated templates escape untrusted values for their statically determined output context and reject unsafe ambiguity at generation time.

```yaml
contexts:
  html_text: escape HTML special characters
  html_attribute: quote and escape attribute value
  url: validate scheme and percent-encode components
  javascript: emit JSON-compatible literal only
  css: allow restricted literal values only
trusted_types:
  - HTML
  - URL
  - JS
rules:
  - trusted types require explicit constructors and are never inferred from string
  - unsafe URL schemes are replaced or rejected
  - dynamic tag names, attribute names, and script source are rejected
  - raw output helper is absent
  - helper return values retain declared trust type
tests:
  - compare accepted subset output with host html/template fixtures
  - include cross-context XSS vectors
```
