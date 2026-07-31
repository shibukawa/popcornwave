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
second_probe:
  call: htmlbind.HasLiveBlock(wrappers, leaf)
  answers: whether the screen keeps changing after this response ends, which is a different question from whether the response needs the boundary runtime at all
  subset: a live block implies an await block, so this probe never changes which branch runs
  used_for: the api:live-delivery-protocol document marker state, and so whether requirement:live-html-rendering costs a live connection at all
gates:
  order: probe the chain, then check configuration, then classify the client
  chain: htmlbind.HasAwaitBlock must report true for streaming to be possible at all
  configuration: html.streaming in data:html-render-config
  client: api:client-classification, added by requirement:bot-synchronous-render
  rationale: the probe is the cheapest and the only one that can rule streaming out entirely, so nothing classifies a client for a page that could never stream
branches:
  buffered:
    condition: no await block, data:html-render-config disables streaming, or api:client-classification reports a bot
    behavior: current htmlbind.RenderChain into a buffer, Content-Length, compression, atomic error replacement
    note: this branch still renders await boundaries correctly, blocking until every one settles, which is what makes requirement:bot-synchronous-render need no second render path
  streaming:
    condition: chain can open a boundary and the client is a browser
    behavior:
      - set Content-Type before the first write and omit Content-Length
      - range htmlbind.RenderChainAsync with the request context
      - write each htmlbind.Content inside the api:html-boundary-protocol parser framing, then call htmlbind.Flush, which reaches the encoder and then the middlewares response tracker
      - stop the range on a write error
representation:
  fact: an await-capable chain now has two byte representations of one URL
  rule: set Vary User-Agent whenever the probe reports an await block, on both branches
  scope: a chain with no await block has one representation and keeps its unvaried cacheable response
  detail: decision:bot-client-classification
rejected_alternatives:
  separate_writer: a WriteHTMLStream entry point forces the handler to know a template detail
  always_stream: loses Content-Length and compression for every static page
  bot_render_path: a separate synchronous entry point for classified bots duplicates a branch that already blocks until every boundary settles
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
