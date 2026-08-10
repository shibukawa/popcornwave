# What Popcorn Wave needs from tinybind-go for a fasthttp build

Surveyed against **tinybind-go v0.5.1** on 2026-08-10, by diffing the exported
surface of every package against v0.5.0.

**Everything this page used to ask for has shipped.** What follows is the record
of what landed and the one thing left, which is ours rather than yours.

---

## Answered in v0.5.1

**`fasthttpupdate`.** The update surface, mirrored over the fasthttp request
value. Entry for entry it is the same set as `htmlupdate` — 28 exported methods
on each, names and order identical — so there is no subset to explain to anyone
and no gap for a framework to work around.

That answers both asks this page carried. The first was for the computing
entries to read a request they do not own; the second listed the streaming half
as deferred and expected to wait. Both are here.

**The streaming half went further than we asked.** We had written it off as
needing the flusher inversion, and it does — but rather than reproduce the
open-then-write shape, `OpenStream` and `OpenLiveStream` were replaced on *both*
sides by `WriteStream(…, head, fn)` and `WriteLiveStream(…, head, fn)`, taking
the callback. That is the same answer the typed stream got, applied to the delta
stream, and it is why live delivery is available on the second backend at all.

**`updatecore`.** `DeltaStream`, `Failure`, `Registry`, `Reloadable`, `Update`,
`Negotiated`, `Mode` and the rest are now aliases into a shared leaf rather than
two declarations. This is the thing we asked for a sentence about in the
framework-owner guide, done properly instead: there is nothing left for a
framework to get wrong, because there is only one of each type.

**`NewStream` removed rather than deprecated**, which is what §4 of the previous
version of this page argued for. The typed stream is now callback-only on both
transports.

**The failure hook takes a context.** `OnFailure` was `func(*http.Request,
Failure)` and is now `func(context.Context, Failure)`. The module reads nothing
transport-shaped to call it, so it stopped asking for something transport-shaped
to call it with — the same move that made the entries portable, applied to the
hook. `Reloadable.Render` moved the same way.

Our side followed all of these; nothing here is outstanding.

---

## What is left, and it is ours

`pwfast` now has the update surface — `WantsUpdate`, `WriteUpdate`,
`WriteUpdateNavigate`, `Redraw`, `RedrawComponents` — over `fasthttpupdate`.

The local blocker we had recorded, that every entry needs options composed from
a config type bound inside `pw`, is solved without moving the configuration
binding: whichever runtime read the configuration file publishes what it
resolved as a transport-free value, and the other half reads it. A settings file
is not a transport concern, so the transport that read it and the transport that
serves the request need not be the same one.

Live boundary delivery is wired, and not through `RenderLiveStream`. We tried
that and withdrew it: our net/http half does not use it either, because it
layers on admission control, a watchdog, digest suppression seeded from the
client manifest, a boundary bound and render telemetry that your entry has no
equivalent for. Calling it would have served a poorer stream on one transport
than the other with nothing to report the difference. So we moved our own
protocol — close reasons, watchdog, admission, keyed digest, manifest parse,
record writers — into a leaf both our halves read, which is the same move you
made for the error types and then for the update types. Nothing is needed from
you for it.

---

## Two notes, neither an ask

**`ApplyTo` still assumes the destination.** It takes `(http.Header,
http.ResponseWriter)`. With `fasthttpupdate` shipping its own response handling
this matters much less than it did, and we apply the header set ourselves on our
side. Mentioned only because the header value itself is portable and the
signature is the one place that does not say so.

**Two `Problem` types.** `bindcore.Problem` is `{Code, Message}` and is the body
your constructors take; a framework on top needs status, title, fields and
cause, so we declare our own. We put ours in a shared leaf both our runtimes
alias, which is the move you made for `FieldError` and then for the update
types. Working as intended on our side — noted because the next framework built
on the module will meet the same fork in the road.

---

## What was never an ask

For the record, since earlier versions of this page got these wrong:

- **The router.** Vendoring `fasthttprouter` beside the fork settled it. Named
  parameters carry over verbatim and only the catch-all spelling is rewritten.
- **Handler arity.** Rewriting both identifiers to one context collapses it as a
  consequence of substitution; nothing needed modelling.
- **A compatibility adapter.** We proposed one and you were right to refuse. A
  buffering adapter preserves neither streaming nor a raw connection, so its
  guarantee was already holed exactly where we need it, and a refusal that names
  the occurrence is worth more than a silent slow path.
- **`htmlbind`.** Every render entry takes `io.Writer`. The heaviest dependency
  we have on the module needed no port at all.

---

## Call registration, on our side

Every `pw` function taking a writer or a request needs a registered call
pattern, or our users get build errors they cannot fix. That work is ours and it
is tracked. The `-transport-report` run over our examples and scaffolds is how
we will know it is complete.
