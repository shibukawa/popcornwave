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

## The badge and the flash

Two things on this page are not deliveries.

`WatchMessages` yields the whole current list, so a delivery cannot say *a
message just arrived* — a reader who was away gets the current room and no
notion of how much of it is new. So the source also emits a **signal**, in the
error slot and classified before anything treats it as an error:

```go
if !yield(nil, pw.NamedSignal("app.message")) {
	return
}
```

No payload: the handler needs to know that something arrived and nothing else,
which lets the server say when and never what.

`handlers/dashboard.pw.html` opens with a `<script component>` block that
receives it. The same block registers two names the runtime dispatches itself —
`pw.live_opened` and `pw.live_closed` — and that is what the badge shows. The
markup renders `connecting…`; only the script says otherwise, so the badge
reports the connection's actual state rather than a claim, and a reader with no
JavaScript keeps the honest one.

The block is extracted to `public/generated/dashboard.script.<hash>.js` at
generation time, so `pw prepare` (or `pw dev`, or `pw build`) has to have run
before the page can load it — the served tree is `dist/public`, derived from the
authored one.

**Watch it work.** Leave the page open. The badge turns `live` when the
connection opens, the room flashes as each message arrives, and stopping the
server turns the badge to `reconnecting…` without the rest of the page moving.

The guides are
[Live Rendering](https://shibukawa.github.io/popcornwave/guides/cross-layer/live-rendering/),
[Signals](https://shibukawa.github.io/popcornwave/guides/cross-layer/signals/),
and
[Component scripts](https://shibukawa.github.io/popcornwave/guides/interactivity/component-scripts/).
