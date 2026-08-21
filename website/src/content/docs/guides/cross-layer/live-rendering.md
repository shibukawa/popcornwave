---
title: Live Rendering
description: Keep one region of a page updating on the server's clock, using a live source and one connection back to the page's own URL.
sidebar:
  order: 2
---

[Async rendering](/guides/cross-layer/async-rendering/) lets a slow section arrive after
the rest of the page. It arrives once. A chat log, a metrics panel, and a
notification feed all want the opposite: the server learns something new, and
one region of a page somebody is already looking at should say so.

Live rendering is that. A template parameter becomes a *source* that keeps
producing, the boundary that reads it re-renders per value, and the browser
holds one connection back to the page's own URL for as long as the screen is
open.

## What changes, and what does not

Nothing about the template syntax changes. A live source is bound in the same
`await` clause an async value is:

```html
external live WatchMetrics(id: string): Point
external async LoadTitle(id: string): string

export component Gauge(id: string): html {
{await title = LoadTitle(id), point = WatchMetrics(id)}
  <h1>{title}</h1>
  <p>{point.label}: {point.value}</p>
{fallback}
  <p>waiting</p>
{/await}
}
```

There is no `{live}` clause and no second terminator, because the wait site
never said how often a value arrives — the declaration does. `LoadTitle`
delivers once and stays; `WatchMetrics` keeps delivering, and every render reads
both. A source that changes from `async` to `live` therefore changes no template
that calls it.

The handler changes nothing at all. A live source is called by generated code
with the subscription's context, so there is no handle to build, nothing to pass
through `Params`, and no streaming API on the response.

## The Go side

```go
// templates/metrics.go
func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error] {
	return func(yield func(Point, error) bool) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				point, err := sample(ctx, id)
				if !yield(point, err) {
					return
				}
			}
		}
	}
}
```

Three properties of that signature are worth naming.

**The context is mandatory.** An `external async` function may take one; an
`external live` function must. A source that never ends has nothing else to make
it return, and a goroutine that outlives its subscription is a leak with no
upper bound.

**A yielded error is a delivery, not the end.** The boundary renders its
`recover` subtree and keeps the subscription; the next good value replaces it
with primary content again. What ends a subscription is the sequence returning.

**Pulling is the backpressure.** The runtime pulls one value at a time, so a
source producing faster than the screen can use it blocks in its own `yield` and
simply misses ticks. There is no queue to size and nothing to discard.

A service that already hands you a channel is adapted by ranging it inside the
sequence:

```go
func WatchMessages(ctx context.Context, room string) iter.Seq2[[]Message, error] {
	return func(yield func([]Message, error) bool) {
		updates := chat.Subscribe(ctx, room)
		for messages := range updates {
			if !yield(messages, nil) {
				return
			}
		}
	}
}
```

## A delivery is the whole region, not the change

`WatchMessages` above yields the **current list**, not the newly arrived
message. That is the rule, and it is what makes everything else simple:

- the render stays a pure function of its inputs, so repeating it is safe;
- a reconnect needs only the next delivery — no replay, no server-held backlog;
- a coalesced or missed delivery costs freshness during a gap and nothing after
  it.

The cost is real too: the subtree re-renders whole, so a long list costs its
length on every delivery. Whoever owns the source owns the accumulation, because
the template holds no state between deliveries.

A source whose values are individually meaningful — one event that must be seen
— is the wrong shape here, precisely because a fast source coalesces. Yield the
accumulated state instead, and send the event as a
[signal](/guides/cross-layer/signals/), which is the channel that exists for
exactly this and is never coalesced.

## What the first response does

Nothing about the first response is special. The document streams exactly as an
async page does: the shell and every fallback commit, and each boundary is
replaced as it settles. A live boundary takes its **first** delivery as an
ordinary completion and then unsubscribes, so the document still finishes.

Two consequences follow.

The first paint shows real content rather than a loading state, whenever the
source produces something within `html.async_timeout`. If it does not, the
committed fallback stays — and that is not a failure, so no `recover` subtree
renders and nothing reaches the error log. A source with nothing to say yet has
not gone wrong.

A crawler, a feed reader, and a browser with JavaScript disabled therefore see
one real render of every live region rather than a permanent placeholder. One
template serves both audiences.

## What the browser does

The document ends with an inert marker saying whether anything more is coming.
When it says live work remains, the runtime opens one connection:

```text
GET /dashboard/7
Pw-Response-Mode: live
```

Same URL, same route, same handler, same authentication. The route runs again,
writes no document body, and streams one record per delivery for as long as the
subscription lives.

That the page executes again is the whole design: it is how a live binding gets
its arguments back without any token, capability, or client-held state. Boundary
ids name positions in the render tree, so the same page executed again produces
the same ids, and a delivery addresses a placeholder the browser already has.

Reconnecting is therefore the same request as connecting. The runtime does it
when the server closes a healthy connection at its lifetime bound, when a stream
is cut off, and after a network drop — promptly with jitter for the first case,
with capped exponential backoff for the others. Nothing about the rest of the
page repaints: the body was never transferred.

Two rarer cases are handled bluntly on purpose. A delivery whose boundary id the
page does not hold means the page's **structure** changed — a panel added to a
dashboard somebody has been watching — and placing that correctly means
rendering a document the browser did not render. A deployment that changed
generated code means every id may now mean something else. Both stop the
connection and reload the page once, guarded per URL so a server that keeps
producing the condition cannot cause a reload loop.

## Output, not input

A live boundary's subtree is replaced on the server's clock, while the user is
doing something else. Anything the browser owns inside it is destroyed without
warning: the value of a field, the caret, the selection.

So the rule is that a live region renders output, and generation enforces it. A
`form`, `input`, `textarea`, `select`, or `contenteditable` in a live clause's
primary subtree is a **generation error**, not a warning, because the failure
mode is silent loss of something the user typed. Keep the form outside the
boundary and the live data inside it.

What no compiler can catch is worth knowing:

- a focused link or button inside the region loses focus when it is replaced;
- the region's own scroll position resets, and the page shifts if its height
  changes;
- a playing video or audio element restarts, as does a CSS animation.

Announcement is yours too, because only you know whether an update is worth
interrupting a screen reader for. A gauge re-rendering every second should
announce nothing. A chat log should be `role="log"`, which implies polite. Put
that attribute on an element **around** the boundary — one inside the replaced
subtree is destroyed and recreated with it, which resets the live region.

## Configuration

Every key sits under the same `html` binding as async rendering, and every one
of them depends on `html.streaming`: a buffered document settles its live
boundaries in place and writes no placeholder, so disabling streaming disables
live delivery rather than leaving a browser applying deliveries to nothing.

```toml
[html]
streaming = true
live = true                    # answer live connections at all
live_max_duration = "10m"      # close a healthy connection and expect it back
live_duration_jitter = 20      # spread that lifetime, in percent
live_idle_timeout = "5m"       # close a connection nothing is delivering on
live_max_boundaries = 32       # bound the boundaries one connection serves
live_max_responses = 4         # bound concurrent connections per client
```

`live = false` is a safe dial rather than an outage: every document stays valid
and keeps the content its live boundaries committed, and no client is invited to
connect. Shedding this load never produces an error page.

The bounded lifetime is the least obvious setting and the most useful one. A
connection that lives forever also authorizes forever, survives a deploy it
should not, and pins one client to one instance. Closing it every few minutes
buys all three back, at the cost of one page execution per rollover — which is
why the jitter matters: without it, one restart synchronizes every client and
the herd repeats on every cycle after that.

If your deployment sets `middleware.request_timeout`, it bounds a live
connection too. Keep it above `live_max_duration` or leave it at zero.

## What it costs

A reconnect is one full page execution: the handler, its layouts, and its await
boundaries all run again, and their output is discarded. Capacity planning
should count page executions per second rather than open connections, because
that is what reaches the database.

That execution re-renders every boundary on the page, not only the live ones,
and most of them come out identical. Only the ones that differ are transferred:
each delivery carries a validator of the bytes it put on screen, the browser
returns them on its next connection, and a region whose validator still matches
what the server just rendered is left alone. A streamed document hands the same
validators to the connection it invites, so the first connection of a page view
is as cheap as a later one. Nothing is stored on the server, and a validator
that fails to arrive costs a delivery that was going to be sent anyway.

Two instances can compare validators only if they share a key. Set
`html.update.validator_key` and they do, whether or not you use partial updates.
The key must carry at least 32 bytes of material — a base64 value counts
decoded — or it is treated as absent and each process keys its own, the same
floor partial updates enforce at startup, because a guessable key lets a log or
proxy observer confirm a low-entropy region's content from its validator.
Without a shared key each process keys its own, so a reconnect that returns to
the same process transfers what changed and one that lands elsewhere transfers
everything — which is what every reconnect did before. Traced responses report
`pw.live.suppressed` and `pw.live.suppressed_bytes` when this is working.

Rendering is per client. Ten screens watching one gauge cost ten renders per
tick, because reconstructed inputs and authorization differ per client. Sharing
one upstream across those subscriptions is the source's job — subscribe once
inside `WatchMetrics` and fan out there, which is also where a circuit breaker
belongs. A failing upstream contained there degrades honestly: the boundary
keeps its last rendered content and the page stays correct.

The connection opens on a head record carrying the chain's tags, so a delivery
whose content reaches a component the document never carried installs its
stylesheet before its markup lands. That is the same ordering a navigation delta
holds, and it is why the two now speak one record grammar and are read by one
reader: a delivery is an `await` record, which is exactly what it is — a
boundary id and the markup filling it.

Live responses are not compressed. Flushing every few seconds keeps the ratio
poor, and a long-lived stream mixing personalized content with request-influenced
values offers far more samples to a compression oracle than one document does.

## Failure, and what the reader sees

| What happened | What the reader sees |
| --- | --- |
| The source yields an error | The `recover` subtree, replaced by primary content on the next good value |
| The source ends | The last render stays; the connection closes and does not retry |
| The clause declared no `recover` | The last render stays, the failure reaches your log, and the connection closes |
| The connection drops | The last render stays until a reconnect delivers the current state |
| The server was deployed | One reload |
| Live is disabled | Whatever the document committed, forever, which is a valid page |

Nothing in that table is a broken screen. That is the property worth keeping:
the document is always a complete page, and live delivery only ever makes it
fresher.

## How it works

The mode is a request header rather than a second endpoint, because the page
must render from its own URL: a mode token in the path or the query string would
reach template scope, and a parallel route would duplicate the authorization and
binding that are already generated and already tested. A custom header also
cannot be set by a cross-origin form or link, which is the class of request CSRF
protection worries about. Every live-capable document therefore carries
`Vary: Pw-Response-Mode`, and every live response is `no-store`.

The stream is newline-delimited JSON, one record per line:

```text
{"control":"open","version":"9f1c…"}
{"id":"tb-1","html":"<p>throughput: 41</p>"}
{"id":"tb-1","html":"<p>throughput: 44</p>"}
{"control":"closed","reason":"retry","retry_after_ms":2000}
```

Markup framing exists to survive an HTML parser consuming bytes as they arrive.
Past the initial document no parser is reading, so a record is the ordinary
shape. The `open` record names the build behind the ids; a `closed` record is
always the last thing written, because a clean connection close cannot say
whether the sources finished or a bound ended a healthy response — and those two
deserve opposite behaviour from the client.

One detail is visible in the DOM. A settled boundary used to replace its
placeholder and disappear; now the applied content is bracketed by a pair of
comment nodes carrying the boundary id. That is the address a later delivery
replaces. Comments are inert, invisible to CSS and layout, and bracket a range
rather than wrap it, so a delivery of several top-level elements still needs no
container you never wrote.
