---
title: Streams
description: Sending a response as a sequence of typed events — SSE, NDJSON, or a JSON array, chosen by the client rather than by the handler.
sidebar:
  order: 3
---

Some responses are not a value you compute and send. They are a sequence that
arrives over time: tokens from a model, lines from a job, events from a queue.
Buffering those into one body means the client waits for the last one before it
sees the first.

`pw.NewStream[T]` opens a response you write into repeatedly:

```go
func events(w http.ResponseWriter, r *http.Request) {
	stream := pw.NewStream[ChatEvent](w, r)
	defer stream.Close()

	for event := range source {
		if err := stream.Send(event); err != nil {
			return
		}
	}
}
```

Three calls, and the type parameter carries the rest. `NewStream` negotiates the
wire format and commits the status and headers. `Send` writes one value and
flushes it. `Close` finalizes the response.

`defer stream.Close()` is not ceremony. In the JSON-array format the closing
bracket is written there, and a stream that never closes produces a body no
parser will accept.

## The client picks the format

The same handler serves a browser's `EventSource`, a `curl` pipeline, and a
`fetch().then(r => r.json())` without knowing which one called it:

| Format | Media type | Framing |
| --- | --- | --- |
| Server-Sent Events | `text/event-stream` | `data: {…}` followed by a blank line |
| NDJSON | `application/x-ndjson`, `application/ndjson`, `application/jsonl` | one JSON object per line |
| JSON array | `application/json` | one `[…]` document, items appended as they arrive |

Selection runs in four steps, stopping at the first that answers:

1. **`?stream=`** — an explicit override. `sse`, `event-stream`, `events`, and
   `eventstream` select SSE; `ndjson`, `jsonl`, `nd`, and `lines` select NDJSON;
   `json`, `array`, `json-array`, and `jsonarray` select the JSON array.
2. **`Accept`** — the leftmost media type in the header that matches one of the
   rows above wins.
3. **`User-Agent`** — a browser token gets SSE; `curl`, `wget`, and `httpie` get
   NDJSON.
4. **NDJSON**, as the default, because it is the one a shell pipeline can read a
   line at a time.

The query parameter exists so you can look at a stream in a browser address bar
without the browser's own `Accept` deciding for you.

Each format brings the headers it needs. SSE gets `Cache-Control: no-cache`,
`Connection: keep-alive`, and `X-Accel-Buffering: no` — the last so an nginx in
front of the application stops buffering the response it is supposed to be
forwarding. The JSON formats get `Cache-Control: no-cache` and their own content
type.

## When there is nothing acceptable

If the request's `Accept` header rules out every supported representation, the
stream never starts. It answers `406 Not Acceptable` as a
[problem response](/guides/frontend/responses/#errors), and every later `Send`
returns that same error rather than writing a body that contradicts the status
already sent — which is why the loop above can just `return` on error.

This gate reads `Accept` alone. `Accept: text/html` is a 406 even with
`?stream=sse` attached, because the override chooses *among* formats a client
said it would take, and that client said it would take none of them.

## Long-lived responses

`server.write_timeout` defaults to `0s`, and this is the reason. A deadline on
the whole response is a deadline on the whole stream, and a stream that is meant
to stay open for minutes would be cut off mid-sequence. If you set the key for
other routes, remember that it applies to these too.

Every `Send` flushes, so a value reaches the client when you send it rather than
when a buffer fills. What can still hold it is something between you and the
client: a proxy that buffers, or a compressing layer that has not been told to
flush.

## Not the same as progressive HTML

An HTML page that streams — the shell first, then each region as its data
settles — is a different mechanism. That one is decided by the templates you
composed rather than by a call you made, and it is covered in
[Progressive rendering](/advanced/async-rendering/). `pw.NewStream` is for
responses whose *content* is a sequence, and it never renders a template.

## What the OpenAPI document knows

`pw.NewStream[T]` call sites feed the generated document like any other typed
response. The operation is described as a streaming surface with `T` as the
event schema, across every media type the negotiation can select, so a client
generator has something to work from. See
[API Documentation](/productivity/api-documentation/).
