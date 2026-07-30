---
id: flow:initial-streaming-render
type: flow
title: Initial Streaming Render
---
One HTTP response streams the document shell, fallbacks, and resolved HTML boundary patches.

```yaml
flow:
  trigger: initial page request whose composed chain reports an await block
  steps:
    - id: prepare
      action: bind input, authenticate, and start async work with api:async-html-value before rendering
      failure: write api:problem-response and stop, since nothing is committed
    - id: select
      actor: decision:automatic-async-render-selection
      action: probe the chain and choose the streaming branch
    - id: validate
      action: htmlbind assembles the chain and checks required async values before the first byte
      failure: map an unset required value to a 500 api:problem-response and stop
    - id: commit
      action: write the content type, then the shell, merged head, and every fallback, then flush
    - id: settle
      action: run boundary work concurrently under policy:async-render-bounds
    - id: patch
      action: wrap each settled fragment in the api:html-boundary-protocol parser framing, write it, and flush, in completion order
    - id: end
      action: finish when the sequence closes, the context cancels, or a write fails
rules:
  - flush through the middlewares response tracker, which forwards to http.Flusher
  - streamed markup and the client contract belong to api:html-boundary-protocol
  - the head references the runtime of requirement:external-boundary-runtime by src, so no completion carries script
  - render post-commit errors inside the page through a recover clause, never as a new status
  - tolerate proxy buffering by forcing the buffered branch through data:html-render-config
  - content encoding on this branch follows decision:streaming-response-compression
protocol: HTML fragment patches, not RSC payloads
requirement: requirement:async-html-rendering
```
