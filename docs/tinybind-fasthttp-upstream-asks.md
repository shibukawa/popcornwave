# What Popcorn Wave still needs from tinybind-go for a fasthttp build

Surveyed against **tinybind-go v0.5.0** on 2026-08-09, by reading the exported
surface of every package rather than the release notes.

v0.5.0 changed `htmlbind` (cache scoping and component privacy) and `templates`.
**`htmlupdate` and `fasthttpbind` are byte-identical to v0.4.10**, so nothing on
this page moved since it was last checked.

The framework side has been taken as far as it goes without you: `pwfast` now
covers binding, the API and problem writers, the full HTML entry set including
the document shell, and the stream. What is left is below.

---

## 1. Let the update entries read a request they do not own

**This is the only ask that blocks real applications**, and it is smaller than
it first looks.

After the response refactor, the update surface splits three ways by what it
actually needs from the transport:

| Group | Entries | Needs |
|---|---|---|
| Nothing | `WriteNavigate`, `FailureResponse` | already portable |
| **Read only** | `WantsUpdate`, `Negotiate`, `Redraw`, `WriteUpdate`, `WriteUpdateStatus`, `Sequence`, `CSRFToken`, `VerifyCSRF`, `Headers`, `RedrawHeaders`, `StreamHeaders`, `LiveHeaders` | **this ask** |
| Write | `OpenStream`, `OpenLiveStream`, `Render`, `RenderStream`, `RenderStreamAsync`, `RenderLiveStream` | see §2 |

Every entry in the middle group takes `*http.Request` and **writes nothing
through it**. They read the render, build, component and instance headers, the
query, and the method, and they return a `Response` value the caller sends. The
transport coupling is a parameter type, not a behaviour.

Two shapes would both work:

```go
// (a) accept a reader
type RequestReader interface {
    Header(name string) string
    Query() url.Values
    Method() string
}
func (o Options) WantsUpdate(r RequestReader) bool
```

```go
// (b) mirror the group in fasthttpbind, as the bind and write halves already are
func (o Options) WantsUpdate(ctx *fasthttp.RequestCtx) bool
```

(a) is one implementation and no duplication; (b) matches what the module
already did for `httpbind` and `fasthttpbind` and needs no new concept. Either
is far less work than porting the package.

**Why it matters here.** An action handler answering with the regions it changed,
and a component redraw, are the two cases the whole partial-update feature exists
for — and both live entirely in this group. Without it, a project that takes the
second backend loses partial updates altogether, which is most of the reason to
use this framework.

---

## 2. The streaming half, which genuinely needs the flusher inversion

`OpenStream`, `OpenLiveStream`, and the four render entries write through the
response as they go, and `DeltaStream` holds a flusher. This is the part your own
`fasthttpbind-parity-scope` already records as needing reimplementation rather
than adaptation, and we agree — there is nothing to shortcut here.

Consequence on our side: a project on the second backend has **no live delivery**
until this lands. We carry live boundaries as NDJSON on the page's own route, so
this is not a niche feature for us.

No ask beyond what you have already scheduled. Listed so the dependency is
visible.

---

## 3. Two smaller things

**`htmlupdate.ApplyTo` assumes the destination.** It takes
`(http.Header, http.ResponseWriter)`. The header set it copies *from* is an
ordinary map and perfectly portable; only the destination is net/http. Splitting
the value out — or exporting whatever composes it — would let a second backend
apply the same headers without reimplementing the copy. Minor, but it is on the
path of every entry in §1.

**`Problem` is two types.** `bindcore.Problem` is `{Code, Message}` and is the
body your error constructors take. A framework on top needs a richer
application-facing value — status, title, fields, cause — and we declare our own,
which means `pw.Problem` and `fasthttpbind.Problem` are different types with the
same name. We solved it locally by putting ours in a shared leaf both our
runtimes alias, which is the move you made for `FieldError`. Nothing is required
of you; noted because any other framework built on the module will hit it, and a
sentence in the framework-owner guide would save them the discovery.

---

## 4. The stream API, unified — and a proposal for your side

We have removed `NewStream` from `pw` outright rather than deprecating it, and
`WriteStream` is now the only entry. Recording the shape here because you said
the module would go the same way, and this is what it looks like carried through.

```go
// pw (net/http)                     // pwfast (fasthttp)
func WriteStream[T any](             func WriteStream[T any](
    w http.ResponseWriter,               r *fasthttp.RequestCtx,
    r *http.Request,                     fn func(*Stream[T]) error,
    fn func(*Stream[T]) error,       )
)
```

Two things made the pair actually work, and both are worth copying:

**The callback parameter must be one type, not two that match.** We had wrapped
your `Stream[T]` to rename `Write` to `Send`. That reads fine on its own and it
breaks the pair: the two handler bodies then differ by a method name, and a
rewrite table that maps selectors one by one does not cover it. Both halves now
alias `bindcore.Stream[T]`, so the body is the same text and only the signature
line moves. **A wrapper that renames anything is a wrapper that breaks the
transform** — that is the general rule, and it is not obvious until you try it.

**Removing beats deprecating, here.** A deprecated `NewStream` that still
compiles is a call site that silently has no fasthttp counterpart. Deleting it
turns that into a build error at the one place that can fix it. Your own
`NewStream` is deprecated rather than gone, and if you do remove it, this is the
argument for it.

We kept one thing on our side: the `406` for an unacceptable `Accept` is written
through our problem path before your `WriteStream` is called, so a browser gets
the application's error page. Everything after the stream opens is yours.

---

## What is not an ask

For the record, since earlier notes of ours got these wrong:

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

Not an ask, but it is the other half of the contract and it is worth you knowing
we have read it: every `pw` function taking a writer or a request needs a
registered call pattern, or our users get build errors they cannot fix. That work
is ours and it is tracked. The `-transport-report` run over our examples and
scaffolds is how we will know it is complete.
