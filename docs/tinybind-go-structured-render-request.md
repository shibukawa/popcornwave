# Change request: expose a render's statics and dynamics separately

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** `github.com/shibukawa/tinybind-go` v0.4.2
**Date:** 2026-08-08
**Status:** open

## Summary

Every wire format above `htmlbind` transfers assembled bytes, because assembled bytes are all a caller can get. A generated plan already holds the answer — `Static` ops are compile-time constants and `Text`, `Attr`, and their variants are functions — but that separation is consumed inside `Plan.Exec` and never reaches a caller.

One ask, with two smaller ones that came out of the same work:

1. **A render entry that yields statics and dynamics per template unit instead of assembled bytes.** This is the ask. It is what lets a delta stop re-sending markup that has not changed, and what lets a browser update a region without reparsing it.
2. **A cache store on the redraw entry.** `Options.Redraw(w, r, registry)` takes no `htmlbind.Option`, so a `@cache` component redrawn on its own runs its body every time while the same component is cached on the page around it.
3. **Priority for the explicit update flag** your `requirement:partial-update-boundaries` already designs. Not a design ask — a scheduling one, with the reason stated below.

We are not asking for a wire format. `decision:caller-owned-wire-versioning` already puts that on us, and we accept it. We are asking for the fragment to be available *unassembled*, with the same identity and the same escaping decisions the module already makes.

## What we did first, and where it stops

We shipped the cheap half downstream before writing this, so the ask is what is left rather than what we had not tried.

Each live delivery now carries a keyed digest of the bytes it put on screen. The browser stores it and returns it on its next connection, and a boundary whose digest still matches what the server just rendered is not written at all. The same digests ride the terminal document marker, so the first connection of a page view starts from what the document committed.

That removes the largest waste we had. A live connection re-executes the page and re-renders every boundary on it, including the ones that settled once — the cost `requirement:live-mode-plan-slice` describes — and we were transferring all of them on every lifetime rollover and every dropped connection. Now we transfer none of the unchanged ones.

What it cannot touch is the changed ones. A live region that ticks every second changes every second, and every tick pays for its whole subtree. Suppression has no answer for that; only structure does.

### The multiplier is worse than it looks

`Content.AppendJSON` escapes the fragment for a script context as well as a JSON one, which is correct and which we rely on. It also means markup is the expensive part of a record by a wide margin, because every `<` and `>` becomes six bytes:

```
<p>one</p>                         10 bytes as HTML
\u003cp\u003eone\u003c/p\u003e      30 bytes in a record
```

A row of a message list — `<li><strong>…</strong> … <small>…</small></li>`, which is the shape in your own `examples/live_render` — carries twelve angle brackets. On this wire that row's markup costs roughly a hundred bytes before a single character of content, on every delivery, for every row. The statics are the payload and the values are the rounding error.

## Ask 1 — a structured render output

### What we need

For one render, in place of a byte range:

- the **static runs** of each template unit, which generation already knows,
- the **dynamic values** in plan order, unassembled,
- the **slot kind** of each dynamic — text, attribute, boolean attribute, URL attribute, raw — so a client can apply a value without deciding how to escape it,
- a **stable identity** per template unit, so a client can cache a skeleton and a server can send it once.

Nested units nest: a `Component` op yields a child unit, a `For` yields one shared skeleton with one dynamics list per item, an `If` yields the branch it took as its own unit. That is the shape a comprehension needs and the only one that makes a list cheap.

A sketch of what we would emit from it, for the message list above:

```json
{"r":"tpl","t":"handlers.Dashboard#msg/7f3a",
 "s":["<li><strong>","</strong> "," <small>","</small></li>"],
 "k":["text","text","text"]}
{"id":"tb-2","t":"handlers.Dashboard#room/2b91",
 "d":["Room 42",{"t":"handlers.Dashboard#msg/7f3a",
      "d":[["alice","hi","10:00"],["bob","yo","10:01"]]}]}
```

The skeleton record is sent once per connection; the delivery record repeats.

### Why it has to be the module

`Plan.Ops` is `[]Op[P]` and `Op` is an interface with one method that writes. `staticOp[P]` is a string and `textOp[P]` holds a function, and both are unexported. Nothing in the public surface distinguishes them, and nothing can: by the time a caller sees output, `execOps` has already concatenated it.

We considered inferring the split downstream by diffing two consecutive renders of the same boundary. It works for transfer size and fails for everything else — it yields no slot kinds, so it cannot support the DOM half below, and it has to guess and recover when a shape changes. We would rather not build it.

### What we would do with it

**Transfer.** Statics stop being retransmitted. The win scales with the markup-to-value ratio of a component, which the escaping above roughly triples for markup and leaves alone for content, and which class-heavy real markup makes large before that. We would apply the same records to the navigation delta, where a list page has the most to gain.

**Application.** This is the half a caller cannot approximate. Today a delivery is applied by setting `innerHTML` on a template element and replacing a DOM range — a full reparse of the subtree, every time, which is also why we carry form state and preserved islands across the swap. With statics and dynamics the client assembles the subtree once and remembers which text node and which attribute each dynamic landed on. Every delivery after that sets `textContent` or calls `setAttribute` and touches nothing else: no parse, no range replacement, no focus or selection loss, no restarted animations, and no state to carry.

That works with no marker in the initial document, which is the property we care about most. The client knows where the slots are because it built them, so the first delivery after a page load lands the old way and every one after it is direct. For a live counter, a gauge, or a ticker, the second delivery onward is the whole lifetime of the screen.

### Escaping is the part we are asking you to keep

If a client assembles a string, it has to reproduce your escaping rules — including `URLAttr`'s scheme check — and getting that wrong turns a text interpolation into markup injection. We do not want that on our side of the line.

What we want instead is for the **slot kind to travel with the skeleton** and for the module to keep every context decision it makes today:

| Slot kind | Value the module sends | What a client does |
| --- | --- | --- |
| text | the raw value | `textNode.data = value`, or escape when splicing |
| attribute | the raw value | `setAttribute(name, value)` |
| boolean attribute | present or absent | add or remove the attribute |
| URL attribute | the already-checked value, or absent | as an attribute; the scheme check stayed on the server |
| raw | the value, unchanged | exactly as trusted as it is today |

The URL row is the important one. `htmlbind/url.go` decides what a URL attribute may contain, and that decision should not be re-implemented in a browser. Sending the checked value or nothing keeps it where it is.

### One identity, not two

`CachePolicy.ID` is documented as *the component identity plus a fingerprint of its generated plan, so regenerated code cannot read entries written by the previous code*. That is exactly what a skeleton cache key needs, and for exactly the same reason: a template edit must invalidate what a client holds.

`Boundary.ComponentID` carries the same property for a different purpose.

We would rather not introduce a third. If the structured output names its units with an identity that already exists, a client's skeleton cache, a server's output cache, and a boundary's validator all invalidate together on a regeneration, and there is one rule to explain rather than three.

### Interaction with the output cache

They are complementary and we would like them to stay that way. The output cache reuses *execution* keyed on inputs; the structured output reuses *transfer* keyed on the template. `policy:layered-cache` on our side puts it as "input hash controls execution reuse; output hash controls transfer".

The one place they meet: a cached component stores assembled bytes today. If a structured render becomes the normal path, the cache either stores the assembled form and loses the split for cached subtrees, or stores the structured form and keeps it. We would prefer the second, and we raise it now because it is cheap to design for and awkward to retrofit.

## Ask 2 — a cache store on the redraw entry

`Options.Redraw(w, r, registry) bool` takes no render options, so there is no way to hand it a `CacheStore`. Every other path we serve — document, navigation delta, action — reaches `htmlbind` through an entry that takes `[]htmlbind.Option`, and gets the store.

The result is a component that is cached when it renders as part of its page and uncached when the same page redraws it alone, which is the case where the cache would have helped most: a redraw exists to re-render one region cheaply.

An options variadic on `Redraw`, matching `RenderStreamAsync`, would close it. We are not asking for a store on `Options` — the store belongs to the caller per render, and `WithCache`'s own documentation says so.

This is small, and we mention it only because we hit it while wiring the cache and there is no workaround on our side.

## Ask 3 — priority for the explicit update flag

`requirement:partial-update-boundaries` already designs it: `declaration.activation.explicit: update flag on an ordinary component`, with the syntax listed as an open question.

Today a boundary is a chain member, so a delta's granularity is the page and its layouts. Changing a sort order on a five-hundred-row table transfers the whole page boundary. `decision:manifest-state-ownership` `size_mitigations` says the right thing — *place boundaries at meaningful regions rather than per list row* — but with no explicit flag there is no way to place one at all.

We are not proposing a syntax; that is yours, and the eligibility rule is the interesting part rather than the spelling. We are saying that of the three things that would reduce what a delta transfers, this is the one that is already designed and unimplemented, and Ask 1 multiplies its value: finer boundaries decide *what* is sent, statics and dynamics decide *how much* each one costs.

## What we are not asking for

- A wire format, a record shape, or a protocol version. `decision:caller-owned-wire-versioning` puts those on us and we are not asking you to take them back.
- A browser runtime. Settled in the v0.3.5 round; ours is ours.
- Changes to boundary identity, the validators, the manifest codec, or the delta operation kinds. All of that works.
- `requirement:live-mode-plan-slice`. It is still the largest cost on the live path and we still want it, but it is a separate piece of work and this request does not depend on it. If anything, the two share a home: both are about a generated plan being executed in part rather than in whole.

## Compatibility

Every item is additive.

- A structured render entry sits beside the byte-writing ones. A caller that does not use it sees no change, and `Plan.Exec` keeps its current path.
- Slot kinds are data on the new output; nothing existing carries them.
- An options variadic on `Redraw` is source-compatible.
- The update flag is opt-in by construction.

We would take Ask 1 behind a `v0.5` and would not expect it in a patch release. It is the largest thing we have asked for and it touches the emitter, not just the runtime.

## What we can contribute

- **A consumer, immediately.** We have both wire formats, a browser runtime we own outright, and a live example with the exact shape this is for. We can implement against a prerelease and report where the output is under-determined, which is what our review of `docs/httpbind_update_wire_contract.md` did for the last round.
- **Measurements.** We can report transfer sizes per delivery on real pages before and after, which is the number that should decide whether the emitter work is worth it. We would rather you had that than our estimate of it.
- **The escaping table above** as a starting point for the slot kinds, and our reasoning for why the URL check must not move.

## Related concepts

**Yours:** `requirement:component-delta-rendering`, `requirement:partial-update-boundaries`, `requirement:component-output-cache`, `requirement:live-mode-plan-slice`, `requirement:streaming-delta-response`, `decision:cache-component-declaration`, `decision:cache-key-derivation`, `decision:generated-render-plan`, `decision:list-item-key`, `decision:caller-owned-wire-versioning`, `decision:manifest-state-ownership`

**Ours:** `api:live-delivery-protocol`, `decision:live-delivery-transport`, `requirement:navigation-delta-rendering`, `requirement:unified-update-runtime`, `requirement:tinybind-update-composition-seams`, `policy:layered-cache`
