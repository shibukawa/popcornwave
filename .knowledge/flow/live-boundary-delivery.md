---
id: flow:live-boundary-delivery
type: flow
title: Live Boundary Delivery
---
From a committed document through continuous deliveries to reconnect, for one page holding at least one live boundary.

```yaml
flow:
  trigger: flow:initial-streaming-render completes for a chain where htmlbind.HasLiveBlock reports true
  steps:
    - id: commit
      action: the document response commits fallbacks and, for a live boundary whose source delivered inside html.async_timeout, its first delivery as an ordinary completion
    - id: mark
      output: the api:live-delivery-protocol document marker, in the live_pending state, as the last bytes of the document
    - id: decide
      action: requirement:external-boundary-runtime reads the marker and opens a live connection only when it says live work remains
    - id: request
      output: the page's own URL with the live mode header and the policy:csrf-protection token, per decision:live-delivery-transport
    - id: authorize
      action: the route authenticates and authorizes exactly as it does for a document request
    - id: execute
      action: the handler, its layouts, and the page run again for the same URL; htmlbind.RenderChainLive writes the document bytes to io.Discard
    - id: subscribe
      action: each live source starts under the request context
    - id: deliver
      action: one delivery re-renders that boundary subtree alone
    - id: send
      output: one newline-terminated record from htmlbind.Content.AppendJSON
    - id: apply
      action: the client replaces the subtree of that boundary id through the same apply function the parser path uses
    - id: loop
      action: back to deliver, for as long as the sources yield and the response lives
    - id: bound
      action: policy:live-subscription-bounds closes the response at its jittered lifetime or on idle, writing the retry record
    - id: reconnect
      action: requirement:live-connection-recovery reissues the same request, which re-executes the page and transfers only live deliveries again
  failure:
    source_failure_delivery: render the recover subtree from pw.AsyncError and keep going; a later value restores primary content
    source_ended: leave the last rendered content and write the done record, so the client stops rather than reconnecting
    no_recover_clause: the failure ends that subscription and reaches the framework, which logs it; the committed content stays
    render_error: keep the current content and report through the error reporter
    handler_error_before_delivery: an ordinary api:problem-response, because nothing is committed
    redirect: a navigate control record, never a 3xx the fetch would follow into a body
    version_mismatch: a reload record, because the ids no longer describe the document on screen
    unknown_boundary_id: the client stops the connection and asks for a reload
    disconnect: every subscription on the response cancels and no source goroutine outlives it
    truncated_document: no marker once parsing finished, which the client answers with a reload rather than a live request
    truncated_live_stream: the stream ends with no terminal record, which the client treats as a reconnect with backoff
    missing_mode_header: the route answers with the ordinary document, which is the decision:live-delivery-transport invariant
```
