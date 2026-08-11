---
title: Signals
description: Send a named instruction from a live source to code the page registered, for the one event that must be seen rather than the state a region shows.
sidebar:
  order: 3
---

A [live source](/guides/cross-layer/live-rendering/) can say one thing: *this
region now shows X*. That is the right shape for a gauge, a queue depth, or a
chat log, and it is the wrong shape for the moment the job finishes.

The difference is not size. A delivery is a snapshot, so a fast source coalesces
and a missed one costs nothing — the next delivery is sufficient by
construction. An instruction is not a snapshot of anything. "The export you
started is ready, go and look at it" is true once, and nothing later makes it
redundant.

A signal is that: a **name** and a JSON payload, sent from a source, dispatched
to a callback the page registered under that name.

```go
// templates/jobs.go
func WatchJob(ctx context.Context, id string) iter.Seq2[Job, error] {
	return func(yield func(Job, error) bool) {
		for job := range jobs.Watch(ctx, id) {
			if !yield(job, nil) {
				return
			}
			if job.Done {
				yield(Job{}, pw.NewSignal("app.finished", finished{URL: job.ResultURL}))
				return
			}
		}
	}
}
```

```js
// This lives in the component's <script component> block.
export function setup(el, scope) {
	scope.on("app.finished", (event) => {
		window.popcornwave.navigate(event.url);
	});
}
```

The client half belongs in a [component
script](/guides/interactivity/component-scripts/): `setup` runs for each
rendered instance, and the scoped registration is removed when that instance is
replaced. This matters especially for a signal handler because partial and live
updates may create the same component more than once during one document visit.

## Why the error slot

A signal is yielded where an error would be, and it is not an error. That is
deliberate, and it is the same move `fs.SkipDir` makes: the second slot is the
one channel every layer already forwards, so a control value placed there needs
no new parameter, no second return, and no change to a signature the template
language already fixed.

It also gets ordering for free. The values and the signals come out of one
sequence, so a signal yielded between two deliveries **arrives between them**,
and a handler that reads the DOM sees the render the source meant it to see. A
side channel — an emit callback, a context accessor — would have needed the
runtime to invent an interleaving.

Three consequences follow from a signal being classified before anything treats
it as a failure.

**It renders nothing.** No `recover` subtree, no boundary content, no revision.
The region on screen is exactly as it was.

**It ends nothing.** The subscription lives, the response stays open, and the
next delivery arrives normally. A source ends its stream by returning, as it
always did.

**It is never coalesced.** Backpressure blocks a fast emitter in its own `yield`
rather than dropping what it produced — which is the whole reason to reach for a
signal instead of another delivery.

## Naming

A name is a lookup key: at most 64 bytes, starting with a letter, then letters,
digits, dot, underscore, or hyphen. Compared byte for byte, so the page's
registration and the source's emission are the same string or they do not match.

Two prefixes are reserved and refused at emit. `tb.` belongs to the template
runtime and `pw.` to this framework — that is where the
[lifecycle names](#lifecycle-names) arrive, and a handler trusts one precisely
because application code has no route into the namespace. Everything else is
yours; a dotted prefix of your own keeps two features from colliding.

## Payload

The payload is a value with a generated encoder, encoded once at construction:

```go
type finished struct {
	URL   string `json:"url"`
	Rows  int    `json:"rows"`
}
```

Nothing inspects it. It reaches the browser exactly as written, which is worth
stating plainly because two habits break here.

**Whatever you put in a payload is public.** No projection happens — a `recover`
clause narrows an error to four safe fields because the runtime defined that
type, and a payload is a struct you named. An internal identifier or an error
string placed in one arrives in the browser.

**A shared source sends to every subscriber.** Fan-out is the application's job
inside the source, which means a source feeding twenty screens from one upstream
emits each signal to all twenty. A shared delivery is usually shared data by
construction; an instruction is usually addressed to somebody. Anything
user-specific comes from a per-subscription source.

## Delivery is best effort

A signal produced while no connection is open is not held, a reconnect replays
nothing that happened during the outage, and the server never learns whether the
browser dispatched one.

So an instruction that must be seen exactly once does not belong here. The
practical test is whether a reader who reloads the page still finds out. A job
page must say "finished" in its own render; the signal is what saves them the
reload.

The budget is `html.live_max_signal_bytes`, 256 KiB of payload per response by
default. Reaching it closes the response for retry rather than dropping records
— a reconnect re-executes the page and the source says the current thing again,
where a dropped instruction is simply gone.

## Lifecycle names

The runtime dispatches into the same table under the `pw.` prefix, for arrivals
the browser is the only party that observes:

| Name | Fires when | Carries |
| --- | --- | --- |
| `pw.document_committed` | the streamed document ended | `final`, `live_pending`, or `failed` |
| `pw.document_truncated` | parsing finished with no end marker | nothing |
| `pw.boundary_settled` | an await boundary's content is in the DOM | the boundary id |
| `pw.live_opened` | the live response began yielding | whether it was a reconnect |
| `pw.live_closed` | the connection ended | the reason and any retry hint |
| `pw.delivery_applied` | a live delivery landed | the boundary id, and whether the DOM changed |
| `pw.navigation_applied` | a navigation delta was applied | the URL now displayed |
| `pw.directive_received` | a navigate or reload directive arrived | which, and the target |

Each fires **after** the thing it describes is in the DOM, because the use is
reading or decorating what just arrived. `pw.document_truncated` is the
exception: it describes an absence, so there is nothing to fire after.

`pw.delivery_applied` carrying `changed` is worth knowing about. The server
skips a delivery whose content matches what your screen already holds, and the
browser leaves an identical one alone, so an arrival is not a change. A handler
that flashes a region reads the flag; a counter that treats arrivals as source
output will be wrong in both directions.

## What a handler may be

A name resolves against the page's table and against nothing else — never
`eval`, never a lookup on a global, never an attribute the payload names. That
restriction is what lets the whole feature exist under
`script-src 'self'` with no `unsafe-inline` and no nonce: what crosses the wire
is a key and some data, and the code was always on the page.

Which puts the real question at registration rather than at dispatch. Matching a
name is checked; what a handler does with an arbitrary payload is not. These two
publish the same name and grant very different things:

```js
scope.on("app.finished", () => window.popcornwave.navigate("/exports/latest"));
scope.on("app.finished", (event) => window.popcornwave.navigate(event.url));
```

The first lets the server say *when*. The second lets it say *where*, which is
an open redirect the moment that URL comes from a row somebody else can write.
Prefer closing over the answer; where the destination genuinely varies, validate
it at the call site.

## In a project that also builds for fasthttp

Call `pwruntime.NewSignal` rather than `pw.NewSignal`, and likewise
`pwruntime.NamedSignal`. They are the same functions — `pw` re-exports them —
but a source is the one part of a live page that names no transport, so it lives
in a file no build tag excludes, and `pw` is not in the fasthttp build. A source
that reaches for `pw` fails that build on the import alone.

Nothing else about a signal changes. Both backends write the same records, both
reserve the `pw.` prefix, and the page's own script does not know which one it
is talking to. See [Build targets](/guides/architecture/performance/) for what
the second build is.

## When not to use a signal

Not for state. Anything a region displays is a delivery — a signal that carries
display state is the wrong shape and gives up the reconnect behaviour that makes
live rendering safe.

Not for anything the client can already tell. A handler that wants to know a
delivery landed wants `pw.delivery_applied`, not a signal your source emits
alongside every value.

Not as a general remote-call channel. The set of things a page can be told to do
is fixed at build time and is exactly what its table holds. A handler that
dispatches on something in the payload has turned one registration into all of
them.

The client-side lifecycle and teardown rules are covered in [Component
scripts](/guides/interactivity/component-scripts/).
