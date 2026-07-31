# Live rendering

One page whose two regions keep changing on the server's clock, and one static
panel that proves nothing else moves.

```bash
pw dev
# or: go run ./cmd/live_render
open http://localhost:8080/
```

The gauge re-renders every second, the room list grows every few seconds, and
the panel above them is delivered once with the document and never sent again.

## What to look at

`handlers/dashboard.pw.html` declares the two sources and binds them in
ordinary `await` clauses:

```
external live WatchThroughput(): Sample
external live WatchMessages(room: string): Message[]
```

There is no live clause and no live handler. `handlers/dashboard_handler.go` is
five lines and names none of this: the sources are called by generated code with
the subscription's own context.

`handlers/sources.go` holds both, and they are deliberately different shapes.
`WatchThroughput` is timer-paced — it decides when a new value exists.
`WatchMessages` is event-paced over a room that every reader shares, which is
where fan-out belongs: the framework renders per client and owns no broadcast
topology.

The room clause also binds `LoadRoomTitle`, an ordinary `async` external, so one
clause holds a value that settles once beside one that keeps arriving. Every
render reads both.

## Things worth trying

**Watch a reconnect.** `config.dev.toml` sets `live_max_duration = "30s"`, well
below the 10 minute default, so the server closes a healthy connection about
every half minute and the browser opens another. The gauge does not stutter, and
the panel above it does not repaint — the body is executed and discarded on a
live request, so only the live regions are transferred.

**Stop the server and start it again.** The page backs off, reconnects to the
new process, and resumes. Nothing reloads: the last render stays on screen for
as long as the outage lasts.

**Turn it off.** Set `live = false` and reload. The page renders once, keeps the
first delivery of each region, and stays a perfectly valid document. That is
what shedding this load looks like — freshness, not correctness.

**Look at it without a browser.** `curl -s localhost:8080/` is classified as a
non-browser client and receives the settled document: one real render of every
live region rather than a placeholder.

**Watch the wire.**

```bash
curl -sN -H 'User-Agent: Mozilla/5.0 Chrome/140' -H 'Pw-Response-Mode: live' localhost:8080/
```

which is the same request the browser makes:

```text
{"control":"open","version":""}
{"id":"tb-1","html":" <strong>36</strong> requests/s <small>at 09:07:30</small> "}
{"id":"tb-1","html":" <strong>34</strong> requests/s <small>at 09:07:31</small> "}
{"control":"closed","reason":"retry","retry_after_ms":2000}
```

Same URL, same route, same handler — the mode is a header, and the page
executing again is how a live binding gets its arguments back after a
disconnect.

The guide is
[Live Rendering](https://shibukawa.github.io/popcornwave/guides/cross-layer/live-rendering/).
