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
  json_script_data: emit valid JSON that cannot terminate its application/json script element
  css: allow restricted literal values only
trusted_types:
  - HTML
  - URL
  - JS
rules:
  - trusted types require explicit constructors and are never inferred from string
  - unsafe URL schemes are replaced, not dropped, so a URL refused in error leaves a trace rather than looking like an attribute the template never wrote
  - the scheme allowlist covers every attribute the browser resolves as a URL, including the multi-URL forms, and not only href and src
  - an inline data URL is admitted by exact media type and never by an image/ prefix, because image/svg+xml carries script
  - dynamic tag names, attribute names, and script source are rejected
  - raw output helper is absent
  - helper return values retain declared trust type
  - JSON strings escape HTML-significant less-than, greater-than, ampersand, U+2028, and U+2029 characters
  - JSON script output cannot contain a literal case-insensitive closing script sequence
  - generated JSON rejects unsupported runtime-dynamic values instead of using reflection
tests:
  - generated HTML and JSON match deterministic golden fixtures
  - generated JSON matches encoding/json field and scalar semantics for the declared supported subset
  - include cross-context XSS vectors
```
