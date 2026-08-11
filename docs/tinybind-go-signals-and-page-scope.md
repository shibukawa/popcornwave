# Signals and scope lifecycle: who owns what

Surveyed against **tinybind-go v0.5.7** on 2026-08-11, by reading the shipped
source, by measuring the browser behaviour the design rests on, and — for the
status table — by checking the wire itself rather than the design catalog, which
in one place describes more than has shipped.

> **Revised.** The first version of this page was written against v0.5.3 and fed
> into the upstream design, which cites it and corrects four of its assumptions.
> Those corrections are marked **[corrected]** below and are worth reading on
> their own — each was a plausible conclusion that does not survive contact with
> what the module already ships.

A **signal** is a named instruction a live source sends to client code. It
travels beside the deliveries a live boundary renders, replaces none of them, and
carries a name and a JSON payload. The client looks the name up in a table the
page registered and calls what it finds. Nothing about the instruction is
transferred but the name and its data, which is what lets a page keep
`script-src 'self'` with no nonce and no `unsafe-eval` while still being directed
by the server.

A **scope lifecycle** is the other half: a script declares `setup` and
`teardown` bound to the lifetime of the thing that declared it, so a region
leaving the screen releases what it registered without the author remembering to.

The two compose and are not the same mechanism. A signal reports that something
happened, to a table registered once for the document's life. A lifecycle is a
per-scope registration that must be released when that scope leaves — which has
no form in a name-to-callback table, because that table is keyed by name and
holds one entry per name, not one per region. `setup` is where a scoped handler
registers into the signal table; `teardown` is what unregisters it.

---

## The one rule that decides every row

The module states it in `htmlbind/asset.go`, and every line below follows from
it:

> The division is the one the module keeps everywhere else. **It decides what is
> required and what its identity is; where the bytes are served is the caller's.**

Read the same rule for the browser: **the module decides what a thing *is*; this
framework decides what *happens* to it.** htmlbind never names a global, never
emits a call into a runtime, and never knows a `pw` object exists.

That is not politeness. `decision:update-runtime-convergence` moved the client
half here precisely because a browser half owned by a dependency meant a change
this framework could make alone still needed a coordinated release. Handing
htmlbind a reference to a runtime object — to call `setup`, or to dispatch a
signal — would rebuild exactly that coupling. Upstream reaches the same
conclusion from its own side.

---

## Responsibility table

### The signal channel

| Concern | htmlbind | Popcorn Wave | Why the line falls here |
| --- | --- | --- | --- |
| The carrier: a signal is an `error` in the source's second slot | **owns** | — | It is a property of the sequence shape the module defines |
| Classifying a yield as signal vs. fault | **owns** | — | *Cannot* be done here. See "Things that are easy to get backwards" |
| `Signal` type, constructors, `AsSignal` | **owns** | thin aliases | The sealed accessor is what makes recognition safe |
| Name grammar and length cap | **owns** | — | It is a dispatch key, and the module bounds it |
| Reserved prefix `tb.` | **owns** | — | Guarded in its own constructors |
| Reserved prefix `pw.` | — | **owns** | A constructor runs at a yield site and is not render-scoped, so the module cannot reach a configured prefix. Each layer guards its own |
| Payload encoding | **owns** | — | Encoded at construction by the generated codec |
| Payload *contents* and who may see them | — | authoring rule | Nothing inspects it; fan-out across subscriptions is the leak to watch |
| Per-record size cap | **owns** | — | Stated with the record |
| Per-response aggregate byte budget | — | **owns** | A live response's cost is bounded here, where every other live bound lives |
| Wire record framing | — | **owns** | The module writes no headers and chooses no framing |
| Which streams carry a signal | — | **owns** | The live stream today; the delta and action paths are this framework's to extend |
| The client dispatch table and its API | — | **owns** | The module explicitly declines to specify the caller's API |
| Lifecycle signal names and their firing moments | **specifies** | **produces** | Each name describes an arrival, and the client is what observes one |

### The scope lifecycle

| Concern | htmlbind | Popcorn Wave | Why the line falls here |
| --- | --- | --- | --- |
| Where an author writes a component's script | **owns** | — | `<script component>` at the top of a declaration, beside the head block |
| That the block is read as JavaScript, not markup | **owns** | — | A raw-text parser rule, so a brace is not an interpolation |
| That a component script must be a module | **owns** | — | A classic script's only per-visit behaviour is re-execution, which is the bug the feature exists to remove |
| Extracting the block to a content-hashed file | **owns** | — | The same pipeline a head script already uses |
| **Which component declaration owns an asset** | **owns** | reads | `Asset.Scope`, package-qualified since v0.5.7. Empty means document lifetime |
| **Which rendered element is an instance of it** | **owns** | reads | `data-<P>-component`, static markup on the component's root |
| Which instances are about to be destroyed | — | **owns** | Only the apply loop knows, so release is client-side and needs nothing on the wire |
| Composition order of the chain | **owns** | reads | `Assets()` / `MergeAssets`, outermost first |
| Where files are written and under which URL | option is the module's | **sets it** | `PublicDir` / `PublicURLBase`, now set to this layout's public tree |
| Writing the extracted bytes to disk | reports them | **writes them** | They arrive as artifacts with a public-asset destination; a caller that ignores them serves a 404 |
| Serving those files | — | **owns** | Ordinary public assets |
| The export name, its argument, the teardown convention | — | **owns** | The module reads no JavaScript and specifies none of it |
| When a scope is entered and left | — | **owns** | Only the client watches the DOM |
| Mount and unmount direction | — | **owns** | Outermost first to mount, innermost first to release, walked over the order the module published |
| The module loader and same-origin enforcement | — | **owns** | — |
| The global object name | — | **owns** | Read from configuration, not compiled in |
| A lifecycle for a region that is **not** a component | — | **owns** | The module reports declarations; anything else is this framework's affordance |

---

## How the lifecycle works

```html
export component Counter(label: string): html {
<head>
  <link rel="stylesheet" href="/shared.css" />
</head>
<script component>
  import { formatTime } from "https://example.test/util.js";  // once
  export function setup(el) {                                  // per instance
    el.on("app.tick", (event) => { … });
    return () => { … };                                        // optional teardown
  }
</script>
<div class="counter">{label}</div>
}
```

The marker `component` is **required** — see the correction below. The block's
content is read verbatim, so a brace is JavaScript rather than an interpolation.
Two blocks in one component is a generation error; a relative import specifier is
too, because the extracted file is served from `PublicURLBase` rather than from
the template's directory.

The export is **named**, not default: it is greppable, it says what it is, and a
module that default-exports something of its own is not mistaken for a
lifecycle. The teardown is what `setup` returns rather than a second export,
because it almost always needs `setup`'s own locals — two exports would have to
communicate through module scope, and module scope is shared across every visit,
which is the one place per-instance state must not live.

### The join, which the contract says needs nothing new

The contract states that everything required already exists on the wire:

- the initial render writes the **instance attribute** into the output;
- the update manifest carries **`component_id`** beside every `instance_id`;
- a delta response reports **insert, remove, move, and replace** by instance;
- and as of v0.5.5, `Asset.Scope` says **which component declaration owns a
  script**.

So the client would join asset → declaration → live instances and run `setup`
per instance, with no new marker and no new record.

**Except the third item is not shipped, and that decides how far this goes.** The manifest entry format is
`<instanceId>:<frame>[:<children>[:<parent>]]`, `BoundaryAttr` writes the
instance id and nothing else, and `component_id` appears nowhere in
`htmlupdate/` or `internal/updatecore/`. It is a field of
`data:component-update-manifest`, which is a design concept rather than a
description of the wire. A client holding `Scope: "Counter"` has no way to find
the elements that are Counters, so the lifecycle cannot be per instance yet.
`docs/tinybind-go-scoped-script-requests.md` is the ask.

**A chain member needs no instance identity, which is what ships today.** There
is exactly one instance of the document, of each layout, and of the page per
render, so position in the composition identifies it as precisely as an instance
id would. This framework computes that chain from each layer's own `Assets()` —
per layer rather than merged, because `MergeAssets` deduplicates by content and
two layers declaring identical bytes would collapse into one entry with one of
them never mounted — and sends it outermost-first: as an attribute on the
document marker, and as a response header on a navigation delta, since a delta
body is the module's to write and has no field to add one to.

The client diffs it. A common prefix stays mounted, the outgoing tail releases
innermost first, and the incoming tail mounts outermost first — so navigating
between two pages of one layout does not tear that layout's script down and
build it again, which is the property a set could not express and the reason
this is a chain.

What that leaves for upstream is narrower than it looked: a scoped script on a
nested component with many instances. A page's own script — the case this whole
design started from — works.

### Measured, not assumed

Removing a `<script>` from the head and re-adding it, in a real browser:

| Script kind | Runs before | After removal | After re-adding |
| --- | --- | --- | --- |
| `<script type="module">` | 1 | 1 | **1** |
| `<script>` (classic) | 1 | 1 | **2** |

A module is keyed by URL in a per-document module map and is not re-evaluated. A
classic script is. This is why "delete the head block and re-add it on return"
does not re-run a module — and why doing it with a classic script is worse than
useless: a re-executed `customElements.define` throws `NotSupportedError`, and
every listener the script installs is added again. Upstream cites this
measurement as the reason a component script block rejects the classic mode
outright.

---

## Corrections to the first version of this page

**[corrected] The axis is the declaring component, not the page.** "Page" is a
caller word; the module has fragments, wrappers, and components and no definition
of a page to attach a flag to. Choosing the declaring component makes a page and
a layout special cases of one rule rather than the two supported shapes, and it
reaches an ordinary component inside a page — which the page axis never would.

**[corrected] Position alone cannot decide that a script is a lifecycle.** The
proposal here was "a `<script>` not in `<head>` is page-scoped". It does not
survive: `<script>{RawJavaScript(js)}</script>` and
`<script>window.payload = {JsonForScript(p)};</script>` are shipped, tested
features, so a top-level script is equally the shape of markup carrying an
insertion. An explicit `component` marker is what selects the new reading, and
that also makes the feature strictly additive — an unmarked script anywhere keeps
its current meaning.

**[corrected] The head keeps carrying the reference tag.** The proposal here was
to withhold the head tag for a scoped script so the head would not accumulate.
That conflated two independent things: **the tag's position decides when the file
loads; the block's position in the source decides its lifetime.** A head tag
loading a module once is exactly right, because module evaluation happens once
anyway — what runs per instance is the exported function.

**[corrected] The conservatism of `Assets()` is fixed by keying on the instance,
not by reading per-layer sets.** `Assets()` deliberately reports what a value
*could* require, including a component below a slot that never renders. The
advice here was to build the chain from each layer's own set instead of the
merged one. That fixes the wrapper case and not the component case, because a
conditional component's asset sits in its own layer's set either way. The real
fix is that an instance which did not render has no attribute and no manifest
entry, so nothing mounts: the conservative set is safe to read as a **catalog**,
and never as a mount list.

---

## Status

| Piece | State |
| --- | --- |
| Signal emission, classification, type, reserved `tb.` | **shipped** upstream, v0.5.3 |
| Forwarding loop, signal record, `pw.` prefix guard, byte budget | **shipped** here, both backends |
| Signal table, `registerEvent` / `unregisterEvent`, lifecycle names | **shipped** here |
| `<script component>` block, extraction, diagnostics | **shipped** upstream, v0.5.5 |
| `Asset.Scope` naming the owning declaration | **shipped** upstream, v0.5.5 |
| Writing the extracted file, flat template path | **shipped** here |
| Writing the extracted file, page tree path | **shipped** here, on `routetree.GenerateTree` in v0.5.6 |
| Scope catalog on the wire, marker scan, loader, `setup`, release | **shipped** here |
| Instance-keyed lifecycle, nested components included | **shipped** here, on the v0.5.7 marker |
| Signals on the navigation delta stream (client) | **shipped** here |
| `navigation_applied`, `directive_received` | **shipped** here — the set is complete |

`<pw-page>` and `definePage(hash)` shipped here against the page axis and are
**superseded in granularity**: the right key is the component instance, which the
manifest already carries. They keep earning their place only for a region that is
not a component at all, which upstream explicitly leaves to the caller.

---

## Things that are easy to get backwards

**Classification cannot be done by the caller.** It looks like the ranging loop
could simply check whether the error is a signal. It cannot, in either
direction: a clause that declares `recover` renders the recovery subtree and
returns `keepOpen`, so the value never arrives as an error at all; and a clause
without one does deliver it, but the binding is already torn down by then, so
recognising it downstream revives nothing. The branch has to be inside the pump.

**A signal ends nothing.** It is not a failure, renders nothing, advances no
revision, and leaves the subscription running. A source ends its stream by
returning, as it always did.

**The head is never removed by a delta on its own.** `installHead` only adds. So
anything contributed at the render call lands somewhere a navigation never
clears — which is why retiring a head tag releases nothing: the script it loaded
has already evaluated and owns whatever it registered. Releasing is what the
lifecycle is for.

**Injection at the render call reaches only the head.** `WithHead` contributes
head nodes and nothing else. This mattered when the design still wanted a marker
in the body; it no longer needs one.

**The signal table and the lifecycle are different mechanisms.** The table is
document-scoped by construction, which is correct for it and is exactly the
property the lifecycle does not have. They compose: `setup` registers, `teardown`
unregisters.

**A registered name is a capability.** The name is checked; what the handler does
with an arbitrary payload is not. A handler that closes over its destination
grants one navigation; the same handler reading a URL out of the payload grants
navigation anywhere, and the difference is invisible in the event name. A handler
that resolves the payload against anything — `eval`, a property lookup, a
selector — collapses the whole table back into the script channel the wire
refuses.

---

## Related catalog entries

- `requirement:signal-forwarding-seam` — the server half here
- `requirement:client-signal-registry` — the browser half here
- `requirement:page-scope-emission` — superseded in granularity by the upstream
  component axis; kept for the parts still true
- `rule:client-event-authoring` — what may be published and what may be sent
- Upstream: `concept:signal-channel`, `decision:signal-in-the-error-slot`,
  `requirement:live-signal-emission`, `requirement:client-signal-dispatch`,
  `requirement:runtime-lifecycle-signals`, `rule:signal-payload-trust`,
  `concept:scope-lifecycle`, `requirement:component-script-block`,
  `requirement:scoped-script-declaration`,
  `decision:lifecycle-from-declaration-block`
