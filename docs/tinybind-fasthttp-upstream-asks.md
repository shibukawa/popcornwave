# What Popcorn Wave needs from tinybind-go for a fasthttp build

Surveyed against **tinybind-go v0.5.1** on 2026-08-10, by diffing the exported
surface of every package.

**Everything this page used to ask for has shipped**, and the record of that is
below. Two asks are open. The first — `routetree` emitting only net/http — is
now the single largest thing between this framework and a runnable second
backend, and it appeared on this page only after we built enough of our own side
to reach it. The second is a convenience we have already worked around.

---

## Open asks

Two, found by diffing every exported surface rather than from memory. The first
is the one that matters.

### 1. `routetree` emits net/http only, and it is the whole page tree

`routetree` names `net/http` in `emit.go`, `pagefunc.go` and `registry.go`, and
there is no option anywhere to emit the other shape. Three separate things come
out net/http-only:

- **The decoder.** `EmitDecoder(route, inputs)` writes `r.PathValue(…)` and
  `r.URL.Query()` — reads off the request value itself.
- **The registration.** `emit.go` writes `mux.HandleFunc(pattern, func(w
  http.ResponseWriter, r *http.Request){…})`, against a one-method `Router`
  interface whose method signature names both halves.
- **The accepted page function.** `pagefunc.go` accepts `func(http.ResponseWriter,
  *http.Request)` as one of the two legal shapes for a page func.

So a fasthttp build of a project with a page tree has no routes and no decoders
at all, which is the single largest thing standing between this framework and a
runnable second backend.

The accessors it would need already exist and are already right:
`httpbind.PathValue` and `fasthttpbind.PathValue` carry the same name and take
the transport first, which is exactly the shape a rewrite wants. Query reads
have the same pair. So for the decoder this is a transport option and a
substitution table, not a design question.

Registration needs one decision from you rather than from us: what the emitted
`Router` interface is on the other side. Ours is
`HandleFunc(pattern string, handler func(*fasthttp.RequestCtx))`, and
`pwfast.ServeMux` satisfies it today — it translates Go 1.22 patterns onto the
vendored trie router, including the `{$}` your emitter writes for the root of
every page tree.

We are **not** asking you to rewrite generated output. Generated files are
outputs rather than transform inputs, emitted per backend, so the emitter
choosing its transport is the whole fix.

### 2. The assembled OpenAPI document has no cached public read

`AssembleOpenAPI()` is exported and transport-free, and five of the seven
OpenAPI entries take no transport at all, so almost none of this surface needed
porting. The two that do are `OpenAPIJSON(w, r)` and `SwaggerUI(specURL)
http.Handler`; we use only the first.

We serve it on the second transport by calling `AssembleOpenAPI()` per request,
because `cachedOpenAPI` is unexported and there is no way to observe a fragment
registration from outside the module. It is a documentation endpoint, so the
cost is acceptable and this is working today.

The ask, if you want it: one transport-free cached read — say
`OpenAPIDocument() ([]byte, error)` — which serves any framework on any
transport and makes `OpenAPIJSON` a thin caller of it. Low priority; we are not
blocked.

---

## Confirmed complete, by measurement

Both diffs were taken against v0.5.1 by comparing exported sets, not by reading
release notes.

- **`htmlupdate` against `fasthttpupdate`** — identical, entry for entry.
- **`httpbind` against `fasthttpbind`** — 71 against 64, and every one of the
  seven is the OpenAPI surface above. Nothing else is missing in either
  direction.

---

## One report, not an ask, for `tinygodriver`

`fasthttprouter`'s package documentation describes the catch-all value as
carrying a leading slash — `/files/LICENSE` giving `filepath="/LICENSE"`. The
fork does not do that, and neither does `net/http`: both yield `LICENSE`, and
the empty string for the directory itself.

The behaviour is right; the comment is inherited from upstream httprouter, where
it was accurate. We wrote a test from the comment and it failed against both
implementations, which is how we found it.

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
pattern, or our users get build errors they cannot fix. That work is ours.

An earlier version of this page said the `-transport-report` run over our
examples is how we would know it was complete. That was wrong, and worth
recording because the reasoning is tempting. The report is green when nothing is
refused, and registering a call is exactly what stops it being refused — so
registration alone turns the report green whether or not anything exists to
receive the rewrite. Seven of ours had nothing, and the report said so by saying
nothing.

What the report proves is that no occurrence is refused. Proving the rewrite
compiles takes a second check, against the receiving package, which we now
have.
