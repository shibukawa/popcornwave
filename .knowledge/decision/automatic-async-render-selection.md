---
id: decision:automatic-async-render-selection
type: decision
title: Automatic Async Render Selection
---
api:html-response chooses buffered or streaming rendering from the composed chain itself, so requirement:async-html-rendering needs no handler-visible streaming API.

```yaml
status: accepted
probe:
  call: htmlbind.HasAwaitBlock(wrappers, leaf)
  cost: reads a constant on the generated plan, starts no goroutine, writes nothing
  transitivity: a component that only calls an async one reports true
branches:
  buffered:
    condition: no await block, or data:html-render-config disables streaming
    behavior: current htmlbind.RenderChain into a buffer, Content-Length, compression, atomic error replacement
    note: this branch still renders await boundaries correctly, blocking until every one settles
  streaming:
    condition: chain can open a boundary
    behavior:
      - set Content-Type before the first write and omit Content-Length
      - range htmlbind.RenderChainAsync with the request context
      - write each htmlbind.Content inside the api:html-boundary-protocol parser framing, then call htmlbind.Flush, which reaches the encoder and then the middlewares response tracker
      - stop the range on a write error
rejected_alternatives:
  separate_writer: a WriteHTMLStream entry point forces the handler to know a template detail
  always_stream: loses Content-Length and compression for every static page
error_handling:
  before_first_write: chain assembly and parameter validation errors, including UnsetPendingError, still map through api:problem-response
  after_first_flush: status is committed, so log through api:logger and never rewrite the body
known_gaps:
  fragment_parameter:
    cause: HasAwaitBlock and head merging ignore a Fragment supplied through a caller Params field
    effect: such a page renders buffered and blocks until every boundary settles; output stays correct
    handling: union caller-built fragment flags and Head values when pw exposes a slot-passing API
  compression: decision:streaming-response-compression
```
