---
id: api:live-delivery-protocol
type: api
title: Live Delivery Wire Protocol
---
The record envelope that carries a live boundary delivery after the document is complete, and the control records that say why a stream ended.

```yaml
status: implemented
extends: api:html-boundary-protocol, as its third envelope beside parser_driven and fetch_driven
transport: decision:live-delivery-transport
request:
  method_and_url: the page's own, unchanged
  headers:
    mode: the live token, which is what selects this response; from decision:update-runtime-convergence it rides the shared render header as 'live;v=N' rather than a header of its own, and pw tests it before delegating to the update modes
    csrf: the policy:csrf-protection header token, as for any credentialed non-document request
  credentials: ambient, exactly as for the document request
  body: none
  carries_no_state: no boundary ids, revisions, or component arguments, because the server reconstructs by executing the page
response:
  status: 200 once the route decides to serve live; a failure before that keeps its ordinary api:problem-response status, since nothing is committed
  content_type: one media type for the delivery stream, distinct from text/html
  headers: Vary on the mode header, Cache-Control no-store
  framing: one JSON record per line, each written with htmlbind.Content.AppendJSON and terminated by a newline
delivery_record:
  shape: '{"id":"tb-1","html":"…"}'
  produced_by: htmlbind.Content.AppendJSON, which escapes the fragment for a script context as well as a JSON one
  meaning: replace the subtree of that boundary id with this HTML
  no_script: no record carries script, so policy:security-response-headers still needs no nonce
control_records:
  open: '{"control":"open","version":"…"}', the first record, naming the generated build behind the ids; a client holding another build's ids reloads instead of applying anything
  closed_done: '{"control":"closed","reason":"done"}'; every source ended, or the page had nothing live, so the client stops
  closed_retry: '{"control":"closed","reason":"retry","retry_after_ms":2000}'; a policy:live-subscription-bounds lifetime, an idle bound, or a refusal ended it, and the client is expected back
  reload: the generated version is incompatible with what the client holds
  navigate: the page returned a redirect, which must not be a 3xx a fetch would follow opaquely into a body
  terminal: one of the closed records is always the last thing written
  version_source: the build's own vcs.revision stamp, so two instances of one deployment agree and a restart evicts nobody; a build with no stamp sends an empty version, which disables the check rather than reloading every client on every restart
  unknown_record: ignored rather than fatal, so an older client keeps a connection a newer server can still serve
document_marker:
  why: no transport signal distinguishes a finished response from a truncated one, and a truncated HTML document still fires DOMContentLoaded and load with nothing surfaced to the page
  written_by: the framework, when the htmlbind sequence exits, after every completion it wrote
  form: '<tb-stream-end state="live" version="…"></tb-stream-end>', inert and carrying no script, as the last bytes of the document response
  branch: the streaming branch only; a buffered document holds no placeholder and no unsettled boundary, so it needs neither the invitation nor the truncation signal, and stays byte-identical to what it was before live boundaries existed
  states:
    final: nothing more is coming; the client opens no live connection
    live_pending: htmlbind.HasLiveBlock reported true, so a live connection is expected
    failed: the sequence ended on an unrecovered failure, per decision:unhandled-boundary-escalation; the committed fallback will not be replaced by this response
  derivation: whether live boundaries exist is known before rendering; whether the sequence ended normally is known where the marker is written
  absence: a streamed document that reaches readyState complete without one was cut off, per requirement:live-connection-recovery; readyState is the trigger and the marker is the evidence
  cost_control: a client that cannot tell final from live_pending pays a whole page execution per screen that never had a live boundary
identifiers:
  positional: htmlbind ids name a position in the render tree, so a boundary nested inside another is tb-1-1 rather than a number from a flat counter
  repeatable: the same chain executed again produces the same ids, which is what lets a live connection address placeholders it did not render
  reused_per_delivery: a live boundary's subtree hands out the same ids every time, so a long-lived connection accumulates no placeholders nothing will fill
  superseded_work: htmlbind cancels a previous delivery's nested boundaries before reusing their ids, so stale content cannot land in the replacement's placeholder
  breaking: the flat tb-N numbering api:html-boundary-protocol described is gone for nested boundaries, which the fetch-path id rewrite must account for
rules:
  - a record envelope appears only outside a document body; marker framing stays parser-driven
  - the client applies a record through the same apply function the parser path uses, per requirement:external-boundary-runtime
  - a record whose boundary id is not on the page is not applied; requirement:live-connection-recovery decides what happens instead
  - a delivery is applied at most once and never merged with another
  - the last record is terminal, and anything after it is ignored
open_questions:
  - the media type, the control record spelling, and whether a record carries a revision before boundary diffing exists
  - whether the document marker also names how many boundaries the render committed, so a client can report completions it never received
```
