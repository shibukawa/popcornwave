---
id: api:live-delivery-protocol
type: api
title: Live Delivery Wire Protocol
---
The record envelope that carries a live boundary delivery after the document is complete, and the framing records that open a stream and say why it ended. Since 2026-08-09 it is the same grammar the navigation delta uses, so one reader serves both.

```yaml
status: implemented
extends: api:html-boundary-protocol, as its third envelope beside parser_driven and fetch_driven
transport: decision:live-delivery-transport
request:
  method_and_url: the page's own, unchanged
  headers:
    mode: the live token, which is what selects this response; from decision:update-runtime-convergence it rides the shared render header as 'live;v=N' rather than a header of its own, and pw tests it before delegating to the update modes
    csrf: the policy:csrf-protection header token, as for any credentialed non-document request
    manifest: Pw-Live-Manifest, id and validator pairs naming what this screen is showing, so the response omits what has not changed
  credentials: ambient, exactly as for the document request
  body: none
  carries_no_server_state: no boundary ids the server issued as a handle, no revisions, no component arguments, and no continuation; the server still reconstructs by executing the page
  manifest_is_a_hint:
    shape: a boundary id, a colon, its validator, and a comma between entries; the same pair form the update manifest uses, in its own header because boundary ids are positional and update instance ids are not
    trust: none needed; every value is compared against a validator this process computes from bytes it just rendered, so a forged entry can only match by being right
    malformed: skipped rather than refused, because a proxy that mangles a header must not become an outage
    bounded: at most html.live_max_boundaries entries are read, since a response cannot serve more boundaries than that
    absent: every boundary is delivered, which is what every connection did before the manifest existed
response:
  status: 200 once the route decides to serve live; a failure before that keeps its ordinary api:problem-response status, since nothing is committed
  content_type: one media type for the delivery stream, distinct from text/html
  headers: Vary on the mode header, Cache-Control no-store
  content_coding:
    what: negotiated per policy:response-content-encoding, and opened at the first delivery rather than with the headers, so a stream that ends before delivering anything writes its close record unframed
    worth: a reconnect, where the manifest suppresses the boundaries whose bytes the client still holds and everything left is a boundary re-transferred whole
    cost_is_unlike_the_other_paths: an encoder here is held for the life of the connection rather than the life of a request, so its buffers scale with concurrent live responses rather than with concurrent requests; a steady trickle of small deliveries also compresses poorly, because each flush ends a block
    revisit_if: a deployment holding many idle live connections finds the encoders rather than the subscriptions are what bounds it, at which point the coding belongs behind its own switch or a smaller window
  framing: one JSON record per line, terminated by a newline, in the record grammar the navigation delta uses
delivery_record:
  shape: an await record naming a boundary id, its markup, and the validator of those bytes
  vocabulary_converged: 2026-08-09, onto the record grammar the navigation delta uses, so one grammar and one reader serve both streams; a delivery is the await record because that is what it is, a positional boundary id and the markup filling it, which is the same operation a settled await boundary lands through
  written_here_rather_than_delegated: the module's live entry writes every completion, and this framework's suppression lives in its own loop over the deliveries; delegating would trade the suppression for the writer, and the wire is this side's to write anyway
  escaping: the fragment is escaped with the module's own JSON string encoder, which is safe for a script context as well as a JSON one
  validator_is_an_extension: the v field is this framework's own beside the emitted shape, which is what a caller owning its wire is expected to add
  meaning: replace the subtree of that boundary id with this HTML
  validator:
    what: a keyed digest of the delivered bytes, which the client stores and returns on its next connection
    keyed_why: it travels in a request header on every reconnect, and an unkeyed digest there is a stable fingerprint of the region's content that a live region with few possible renderings makes enumerable from a proxy log
    key: api:html-update-options validator_key when a deployment configured one, read whether or not updates are enabled, because only a shared key compares across the instances a reconnect may land on
    key_fallback: a per-process key, which narrows suppression to a reconnect returning to the same process rather than reintroducing the fingerprint
    absent: no key at all disables suppression rather than falling back to an unkeyed digest
    length: twelve bytes, base64url; the digest decides only whether to skip a transfer the client would have discarded
  suppression:
    rule: a delivery whose validator matches what the client claims, or what this response already sent for that boundary, is not written at all
    why_not_a_record: the client discards an identical delivery on arrival, so the record buys only the bandwidth — which on a reconnect is every boundary on the page
    idle_bound: a suppressed delivery still counts as activity, because the source produced a value and closing the stream would cost a page execution to learn the same thing again
    observability: pw.live.suppressed and pw.live.suppressed_bytes on the render span, because a delivery count alone cannot distinguish a quiet page from one whose every delivery is skipped
  no_script: no record carries script, so policy:security-response-headers still needs no nonce
control_records:
  head: the first record, carrying the build behind the ids and the chain's head tags; a client holding another build's ids reloads instead of applying anything
  head_carries_tags: a delivery whose content reaches a component the document never carried needs that component's tags before its markup lands, which is the ordering the navigation delta makes normative and which this path had no channel for at all before the vocabulary converged
  build_fallback_differs_from_an_update: an unstamped binary reports nothing here, which disables the check rather than reloading every open screen on every restart; an update falls back to the per-process identity instead, because there a wrong delta costs more than a re-transferred page
  end_done: the terminator when every source ended, or the page had nothing live, so the client stops
  end_retry: the terminator with a retry hint in milliseconds; a policy:live-subscription-bounds lifetime, an idle bound, or a refusal ended it, and the client is expected back
  reload: the generated version is incompatible with what the client holds
  navigate: the page returned a redirect, which must not be a 3xx a fetch would follow opaquely into a body
  terminal: an end record is always the last thing written, and nothing after it is read
  version_source: the build's own vcs.revision stamp, so two instances of one deployment agree and a restart evicts nobody; a build with no stamp sends none, which disables the check rather than reloading every client on every restart
  unknown_record: ignored rather than fatal, so an older client keeps a connection a newer server can still serve
document_marker:
  why: no transport signal distinguishes a finished response from a truncated one, and a truncated HTML document still fires DOMContentLoaded and load with nothing surfaced to the page
  written_by: the framework, when the htmlbind sequence exits, after every completion it wrote
  form: '<tb-stream-end state="live" version="…" manifest="tb-1:…,tb-2:…"></tb-stream-end>', inert and carrying no script, as the last bytes of the document response
  manifest_attribute:
    what: the validators of every boundary this document committed, seeded into the client's state so the connection it invites starts from what is already on screen
    without_it: the first connection of every page view re-transfers the whole screen, because the document delivered through the parser and the connection that follows cannot know which bytes are there
    live_state_only: a final document carries none, since it would describe a conversation that is not going to happen; a failed one carries none either, because the boundaries it committed are fallbacks a reconnect should replace
    ordering: the marker is the last markup of the document, so every tb-apply has already run and every range it names is in place
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
  - the client stores the validator it was given and returns it, and never computes one; only the server can say whether one still holds
  - a claimed boundary whose range has left the document is not claimed, because an enclosing boundary re-rendered and took it along
answered:
  record_carries_a_revision: yes, and before boundary diffing rather than with it. The validator is what a delivery is worth skipping over, and skipping a whole boundary is the cheapest form of the diffing this entry was waiting for. A later static and dynamic split grows the same record rather than replacing it.
open_questions:
  - the media type and the control record spelling
  - whether the document marker also names how many boundaries the render committed, so a client can report completions it never received
  - whether a boundary's static markup can stop being retransmitted at all, which is the next reduction and needs a structured render output system:tinybind does not expose
```
