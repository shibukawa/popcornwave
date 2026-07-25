---
id: flow:initial-streaming-render
type: flow
title: Initial Streaming Render
---
One HTTP response streams the document shell, fallbacks, and resolved HTML boundary patches.

```yaml
flow:
  trigger: initial page request
  steps:
    - validate route, authentication, and critical input before first flush
    - send document shell and unresolved fallbacks
    - continue cancellable server work
    - append ordered HTML fragments and replacement instructions
    - end after required boundaries resolve or cancel
rules:
  - use http.Flusher when supported
  - embed executable instructions only with CSP nonces
  - render post-flush errors inside the page
  - tolerate proxy buffering and offer non-streaming fallback
protocol: HTML fragment patches, not RSC payloads
```
