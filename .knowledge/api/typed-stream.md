---
id: api:typed-stream
type: api
title: Typed Stream API
---
pw.NewStream negotiates and writes a typed event stream through one API.

```yaml
surface:
  - NewStream[T](http.ResponseWriter, *http.Request) Stream[T]
  - Stream.Send(T) error
  - Stream.Close() error
representations:
  - text/event-stream
  - application/x-ndjson
  - application/ndjson
  - application/json
negotiation:
  source: Accept header
  unsupported: HTTP 406 problem response
implementation: wraps the TinyBind streaming facility behind pw
```
