# websocket_chat

A chat room over a typed WebSocket, on one port, in both builds.

Two structs are the whole protocol. `ClientMsg` is what a browser sends and
`ServerMsg` is what the room answers with, and generation writes a decoder for
the first and an encoder for the second — so the handler reads and writes
structs and never touches a byte.

```bash
pw generate
go run ./cmd/websocket_chat
```

Then open <http://localhost:8080> in two tabs and type in both.

## What to read

`handlers/chat.go` is the protocol: two structs, each carrying its variants in a
`type` field. The library names nothing there, because the protocol is the
application's.

`handlers/chat_handler.go` is the endpoint. `pw.WebSocket` upgrades the request,
runs the loop, and closes the socket whichever way the loop ends. Neither type
argument is spelled at the call — generation recovers both from the closure
parameter, which is why `pw generate` is not optional here: a socket whose types
were never discovered opens and then fails on its first message.

`handlers/hub.go` is the room, and it is the interesting half. The framework
owns one connection — its read limit, its deadlines, its ping cadence, its close
handshake — and owns no registry of connections, because who is in the room is
the application's question. So this file exists, it is 60 lines, and it names no
transport package at all: a member is anything that can be written to, and the
socket both builds hand the callback satisfies that.

That is also what makes `broadcast` safe. It writes to every member from one
goroutine while each of those members has its own reader running, and the socket
serializes writes so a frame from one message cannot interleave with another.

## The second build

`popcornweb.toml` declares `fasthttp = true`, so `pw generate` derives the
fasthttp half of every handler:

```bash
pw generate
go build -tags fasthttp ./...
```

The derived handler is in `handlers/tinybind_transport_pw_gen.go`, and the
callback body inside it is the same text as the one you wrote. Only the import
qualifier moved. That is what the lifetime rule buys: nothing in the callback
reads `w` or `r`, because on this transport the callback runs after the handler
has returned.

## The origin check

The socket refuses an upgrade from another site. A handshake is a `GET` carrying
the session cookie and it never meets the CSRF middleware, so an upgrade
admitted from anywhere would be cross-site request forgery with a connection
left open behind it.

`handlers/chat_handler_test.go` drives that refusal along with the room itself:
two clients join, one speaks, the other hears it.

```bash
go test ./...
```

Behind a TLS-terminating proxy, declare it as `server.trusted_proxies` — the
comparison includes the scheme, and an undeclared proxy makes the deployment
resolve its own origin as `http://` while the browser reports `https://`.

## Where a failure goes

Once the connection is accepted there is no status left to carry an error, so
`cmd/websocket_chat/main.go` installs `pw.SetStreamErrorHandler`. The name says
stream because one installer covers both; without it, a socket failing in
production says nothing at all.
