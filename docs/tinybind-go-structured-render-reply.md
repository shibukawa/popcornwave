# Reply: verification, measurements, and three answers

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**To:** `github.com/shibukawa/tinybind-go`
**Against:** your proposal of 2026-08-08, answering our request of the same day
**Status:** reply. Two of our claims withdrawn, your sequencing accepted, measurements below.

## Summary

We read every claim in your proposal against v0.4.2 and rendered the ones that could be rendered. **All of them hold.** Two of ours did not, and we withdraw them.

Your sequencing is accepted: render options, then the update flag, then the structured output. Ask 2 is worse than you found — we reproduced it against our own entry points and it is a live 500 on a documented path, with no workaround available downstream.

Your three questions are answered in Part 3. Short forms: the measurements are in Part 2 and support the emitter work; the element description is accepted with one question back; component-per-row is accepted, and we think it improves our templates rather than taxing them.

## Part 1 — What we verified

### 1.1 Your two rendered claims reproduce exactly

Rendered against v0.4.2, not reasoned about. Output and error text match yours character for character.

```
CSRF   no options              -> err: htmlbind: form needs a CSRF token: this render
                                       supplied none; pass WithCSRFToken, or
                                       WithoutCSRFToken for a render with no session behind it
       WithCSRFToken("tok")    -> <form method="post"><input type="hidden" name="_csrf" value="tok"></form>

URL    no options              -> <a href="#tb-blocked-url">x</a>
       WithURLSchemes(…,myapp) -> <a href="myapp://open/42">x</a>
```

We also rendered two claims you stated rather than measured, and both hold. An absent optional attribute leaves nothing behind — `<article>x</article>` against `<article title="t">x</article>` — so presence is structure, as you say. And a `Text` closure returns `a<b&c` while the render writes `a&lt;b&amp;c`, so the text kind is available as data today.

Thirteen render options is exact: eight in `render.go` and five more across `csrf.go`, `head.go`, and `url.go`.

### 1.2 Ask 2 is worse than the report

We reproduced it against `pw.WriteUpdate` rather than against `WriteUpdateStatus`, with a region holding a form:

```
status       = 500
content-type = application/problem+json
body         = {"type":"about:blank","title":"Internal Server Error","status":500,…}
```

Our own guide documents the pattern that fails, in these words: *a rejected submission returns 4xx and the regions it carries are the validation errors — showing them is the point*. Re-rendering a form region with its errors is the worked example, and a form region is what cannot render.

**There is no downstream workaround.** We looked for one. `Update` carries an `htmlbind.Fragment` and nothing else, `writeActionBody` and the delta body types are unexported, and the render happens inside `WriteUpdateStatus`. The only way to fix this from our side is to re-implement the action writer against your published wire contract — which is exactly the duplication the client-ownership round removed, and we are not going to reintroduce it to route around a missing variadic.

So the variadic is not an improvement we would like. It is the fix, and until it lands the documented pattern is unavailable in a released framework. Your raise to must is right and we would take it in a patch release.

Two notes on blast radius, in your favour:

- The URL divergence does not bite us today, because we configure no scheme allowlist. It would bite the first application that does, and silently, since the divergence is stricter.
- The boundary prefix divergence does not bite us either, because our `UpdateAttributePrefix` is `"tb"` and that is `DefaultDataAttributePrefix`. We chose the module default in an earlier round precisely to avoid two spellings in one document, and it happens to mask this. An application that overrode the prefix would find its redrawn boundaries unaddressable by our runtime, which locates by `data-<prefix>-id`.

### 1.3 Your catalog quotations are verbatim

Every one we checked resolves and reads as quoted:

- `decision:dom-application-strategy` is dated 2026-08-01, its staging step 3 is the static-dynamic split, and the content-address sentence is verbatim at line 33. Its open question — *how a skeleton is first delivered: inline with the initial render, or fetched by content address* — is the one this round is about. **Your framing that Ask 1 is your own unnamed half is supported by your own catalog, and we accept it.**
- `decision:generated-render-plan` lists exactly four phase-dependent capabilities, so "a fifth" is exact.
- `decision:url-context-escaper` states the structural reason in its own words: *Attr takes value func(P) (string, bool), so the closure holding the current htmlbind.Escape call has no Renderer and cannot see a render option*. Your §2.5 is that argument applied one step further, and its emitter test already asserts that a `URLAttr` line carries no `Escape` call. The precedent is real.
- `decision:list-item-key` says a match target has to be a node. `decision:author-declared-boundary-id` does distinguish an author-written DOM id from the automatic positional identity, so your Ask 3 question 2 is a real fork.

The generated fixture matches your quote closely enough that we could diff it. One thing your quote drops: `BoundaryAttr()` sits between `Static(" <article")` and `BoolAttr("hidden", …)`. That omission works for you rather than against you — it is a live instance of the `module owned` kind your §2.4 adds, interleaved in the op list exactly where a skeleton would have to describe it.

### 1.4 Two claims of ours, withdrawn

**The action path takes render options.** It does not. We wrote that the document, navigation delta, and action paths all reach `htmlbind` through an entry taking `[]htmlbind.Option`. `WriteUpdate` and `WriteUpdateStatus` take none, our own wrapper passes none, and §1.2 above is what that costs. Withdrawn, and the correction is the reason Ask 2 moved.

**`examples/live_render` is yours.** It is ours. Nothing in the argument depended on it, but the attribution was wrong and we are glad you checked it rather than assuming we meant something you had.

## Part 2 — The measurements

You asked for transfer size per delivery, before and after, and said that number should decide whether the emitter work is worth doing. Here it is, with the method stated so you can disagree with it.

**Method.** We reproduce the Room panel shape of our `examples/live_render` as a plan — a title, then a `for` over messages emitting `<li><strong>{author}</strong> {text} <small>{at}</small></li>` — render it, and compare three numbers. *Today* is the delivery record `Content.AppendJSON` produces. *Projected* is the record a statics-and-dynamics wire would write for the same data, envelope included: every value quoted and comma separated, one array per row, two content addresses, and the skeletons amortized to zero because they travel once per connection. *Positional diff* is what a byte-level diff of two consecutive renders would still have to send.

The event is one new message arriving, which shifts the list by one. That is the ordinary live event on this panel.

| rows | today | projected | ratio | a positional diff would still send |
| --- | --- | --- | --- | --- |
| 5 | 743 B | 262 B | 2.8× | 325 B — 86% of the HTML |
| 30 | 4,039 B | 1,258 B | 3.2× | 2,121 B — 97% |
| 100 | 13,280 B | 4,059 B | 3.3× | 99% |

Two readings.

**The split is worth roughly three times on this shape, and the ratio holds as the list grows.** It does not run away with row count, because values grow with rows too. Three times is our number for a text-heavy panel with short markup per row; markup carrying classes, ARIA attributes, or SVG would do better, and a panel that is mostly one long text value would do worse.

**The alternative we rejected is worse than we argued.** We told you a downstream byte diff yields no slot kinds and would need to guess on shape changes. The measurement says something sharper: for the ordinary live event it saves nothing at all. A prepended row shifts every subsequent byte, so the common prefix and suffix cover 1–3% of the fragment. We had argued against building it on principle; we now have a number saying it would not have worked.

**What this is not.** It is a computed projection on a reproduced template shape, not production traffic. The projected column assumes a record shape neither side has settled, and §3.2 below is where that assumption could be wrong. We can rerun it against whatever shape you land on, and against more template shapes if you name the ones you doubt.

## Part 3 — Your three questions

### 3.1 Component-per-row: accepted, and we think it is an improvement

This is the one place your design asks something of our templates, and our answer is that it asks for something we would want anyway.

A row written inline is a row whose behaviour is invisible: nothing in the template says whether it re-renders as a unit, and a reader has to know the delta rules to work it out. `<MessageRow m={m}/>` says it. A reader of the template can see what the unit of update is, which is the same property that makes an explicit boundary better than an inferred one.

So we will document component-per-row as the way to write a list whose rows update, and we do not need it softened. If it later turns out to force awkward decomposition on a shape we have not met, that is a report we will file with the shape attached rather than a reason to hedge now.

### 3.2 The element description: accepted, with one question back

Your §1.2 is right that `"s"` cannot be a flat array of strings, and we withdraw that part of the sketch. An attribute's static text lives inside its op, and an optional attribute changes the runs themselves, so a skeleton has to describe elements rather than concatenate around holes.

**What we need from the shape**, stated as a property rather than a form: a client must be able to walk the skeleton, construct the DOM, and record where each dynamic landed — as a text node or as an element-and-attribute-name pair — without parsing HTML. That is the whole point of §2.4 of your proposal, and it is what makes the second delivery onward a `textContent` assignment.

**The question back is about the first build, not the steady state.** An element description built through `createElement` and `setAttribute` is exact but slow to construct: a hundred-row list is several hundred DOM calls where `innerHTML` is one parse. The steady state is what we are optimizing and the first build is what the user waits for, so we would rather not trade one for the other silently.

Two ways out, and we would like your read:

1. **The skeleton carries both**: an assembled string form for the first construction, plus the slot addresses within it. The client sets `innerHTML` once and then walks to the recorded positions. Costs skeleton size, which travels once per connection.
2. **The client assembles the string itself** from the element description, applying the escaper each kind names, and then parses once. Costs nothing on the wire and puts an assembler in the client — but only a string assembler, never a policy, since your §2.4 already keeps every judgement on your side.

We lean to 2, because it keeps the skeleton one representation rather than two that could disagree, and because the assembler is mechanical in a way the escaping rules are not. But we would rather hear which one your emitter finds natural before we commit our client to either.

One constraint either way: the `raw` kind forces a parse for that slot, so whatever shape you pick has to express "this hole takes markup, not a value". Your table has the kind; we are noting that it is the one slot the fast path cannot serve.

### 3.3 Ask 2's two open items

**A render reaching `CSRFField` with no token supplied stays a failure.** We are the ones eating the 500 and we still say yes. The alternative is a form rendered with an empty token, which submits, is rejected, and leaves nothing pointing at the cause — the failure mode our own guide already calls out for the non-request render case. The diagnostic names both ways out, which is what makes the failure actionable. We accept that this is a behaviour change on a released path and that it is your decision rather than a default.

**`WriteUpdate` should take the request.** Our `pw.WriteUpdate` already receives `*http.Request`, and it is where both the context and the session's CSRF token come from on our side. A request parameter puts the token and the cancellation in the same place `Redraw` already has them, and it removes the one asymmetry between the two entries. If you would rather keep the signature and have callers pass `WithContext` and `WithCSRFToken` through the variadic, we can do that too — it is strictly more typing at one call site, not a design problem.

Agreed and not contested: the store belongs to the caller per render, not on `Options`.

## Part 4 — One precision point on identity

Your §2.2 satisfies what we asked for, and we want to name the gap between the words and the mechanism before either side builds against the words.

We asked that one identity serve a client's skeleton cache, a server's output cache, and a boundary validator. What §2.2 offers is **one derivation rule producing several related identities**: `decision:cache-key-derivation` is template-path-plus-component-name scoped, one per component, while a skeleton address is per emitted skeleton, so a component with a conditional or an optional attribute has several. You say this yourself — *present and absent are two addresses* — so nothing is hidden. But "one identity, not two" read literally would suggest the values coincide, and they do not.

The property we actually needed is the invalidation one: a template edit must invalidate all of them together, and there must be one rule to explain rather than three. Both hold under your design. We would just rather the catalog said *one rule, several addresses* than *one identity*, so that nobody implements against the stronger reading and finds it false.

## Part 5 — Sequencing

Accepted as you propose it: render options, then the update flag, then the structured output.

We ranked by value and you are right that value was the wrong axis for the first two. A defect on a documented path and an opt-in flag that blocks on nothing should not queue behind a design round, and §1.2 above is us finding out how much the first one costs.

On Ask 3, your three settling questions are the right three. We have no position on the exported-only rule. On the other two our reading matches yours: `reloadable` is client-addressed re-rendering and the update flag is participation in server-discovered deltas, one component should be able to be both, and `reloadable` should not imply the flag — a component that a page can redraw on demand is not necessarily one whose markup a navigation delta should compare. On the identity question, a flagged component taking the automatic positional identity is what our manifest already assumes, and an author-written id would be a second entry shape for us to carry.

## What we will do next

- Land the client half of the structured output once the skeleton shape settles. Our runtime is ours and the wire format is ours, so nothing there needs you.
- Rerun the measurements against the record shape you land on, and against more template shapes if you name them.
- Report the URL and prefix divergences as fixed on our side once the variadic ships, rather than working around them now.

## What is not changing

Unchanged from your list and ours: the wire format and versioning stay with us, the browser runtime stays with us, and boundary identity, the validators, the manifest codec, and the delta operation kinds are not in question. `requirement:live-mode-plan-slice` remains accepted, unbuilt, and separate.
