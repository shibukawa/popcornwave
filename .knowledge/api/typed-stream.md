---
id: api:typed-stream
type: api
title: Typed Stream API
---
pw.Stream negotiates a typed event stream and runs the caller's send loop inside a callback, which is the one streaming shape both net/http and a register-a-writer backend can run.

```yaml
surface:
  - WriteStream[T](w, r, func(*Stream[T]) error)
  - Stream.Write(T) error, which writes one value and flushes it
  - SetStreamErrorHandler(func(error)), shared with the other runtime
as_built_2026_08_09:
  net_http: pw.WriteStream, with NewStream and the Stream wrapper removed rather than deprecated
  fasthttp: pwfast.WriteStream, the same name and the same callback
  one_type: pw.Stream and pwfast.Stream both alias the module's Stream, so the callback parameter is one type and a handler body is the same text on both
  send_became_write: the wrapper renamed the module's Write to Send, and keeping that would have made the two bodies differ by a method name the rewrite table does not cover, which is the appearance of a shared shape without the substance
  discovery: generation registers the new entry, and the module recognizes both names, so a call site is found either way
  docs: the website teaches the callback form in both locales, and the always-close pitfall is gone because the runtime closes
representations:
  - text/event-stream
  - application/x-ndjson
  - application/ndjson
  - application/json
negotiation:
  source: Accept header
  unsupported: HTTP 406 problem response, written before the callback runs
implementation: wraps the TinyBind streaming facility behind pw
callback_shape:
  shipped_upstream: tinybind-go v0.4.9 replaced the open-then-defer-Close entry with a callback on both transports, so this is adoption rather than a proposal
  why: a backend that streams by registering a writer runs it after the handler returns, so a stream value the handler holds and writes to has no equivalent there
  evidence: fasthttp spells its own streaming 'type StreamWriter func(w *bufio.Writer)' with SetBodyStreamWriter, and documents that the writer from BodyWriter must not be used after the handler returns
  net_http: the callback runs inline, and Send writes and flushes through http.Flusher
  register_a_writer_backend: the callback is registered and runs afterward, and Send writes and flushes through the buffered writer it is handed
  same_reason_as: api:redirect-response and every other surface whose portable form is a callback rather than a value the handler keeps writing to
  replaces: NewStream returning a Stream the handler sends on and closes
  closing: the callback returning is the close, so a stream cannot be left open by an early return the way an explicit Close can be missed
  no_return_value: the entry point returns nothing, because on the other transport the callback runs after the handler returned and an error cannot travel back to handler code
  errors: a callback error is post-commit and reaches an installed stream error handler; a failure to open the stream is still an api:problem-response, because nothing is committed yet
  pre_commit_window_dropped:
    intended: defer the commit so a callback failing before its first event could still produce a problem response, which is the common case of a query failing before anything is written
    why_not: the other transport runs the callback after the status went out, so the window cannot exist there, and keeping it on net/http alone would make one callback behave differently per transport
    cost_accepted: a callback failing before writing anything still sends 200 followed by an empty stream
  defects_it_fixed: a missing Close truncating a JSON array document, and the documented discard of write errors, both of which the callback makes unconditional
what_it_covers:
  in: server-sent events, newline-delimited records, progressive chunked output, long polling, and anything else that keeps HTTP framing
  out: a protocol that stops speaking HTTP after an upgrade handshake, WebSocket being that case
  where_that_goes: an upgrader taking the request, not a body writer and not a raw connection, placed in requirement:contrib-websocket
why_the_shape_had_to_change:
  before: an adapter was proposed for what the transform could not take, and it could never have preserved streaming, since a buffering adapter collects the response before writing it
  after: decision:transport-compatibility-fallback records that no adapter was built, so a stream is either written in a rewritable shape or it is a refusal
  effect: this surface is not one option among several; it is the only way a streaming handler reaches the second backend at all
```
