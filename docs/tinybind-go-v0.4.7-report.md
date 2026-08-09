# Two defects in v0.4.7, and one still open from v0.4.6

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** `github.com/shibukawa/tinybind-go` v0.4.7
**Reports:** what the header split cost, and what an unmodified v0.4.7 does to a client that follows the guide
**Answered:** v0.4.8 — all three closed, and a fourth found while closing them. Kept for the record of the round; see the notes below each item for what shipped.

The split is right and we have taken the whole of what it hands over: the cache policy of all four shapes, the conditional request, the refusal body, and the Vary axes. Nothing below is an argument against it.

The first defect comes from the seam the split created, and on its own **it turns the sequence capability off while leaving it looking on**. The second predates the split and empties any list built from reloadable rows. Both are silent: the response is correct as bytes, the resulting DOM is valid, and the runtime reports success.

Neither was reachable from a test. We found them by opening a page.

## 1. A sequence response says it is a navigation

`Headers` computes the echo through `renderToken`, and `modeName` has no sequence case:

```go
// htmlupdate/htmlupdate.go:383
func modeName(mode Mode) string {
	switch mode {
	case ModeLive:   return modeLive
	case ModeRedraw: return modeRedraw
	default:         return modeNavigation
	}
}
```

`ModeSequence` falls to the default. In v0.4.6 the entry wrote `modeSequence` itself; routing the echo through the shared function is what dropped it.

```
$ curl -sD - -o /dev/null -H 'Pw-Render: sequence' -H 'Pw-Sequence-Address: <a real one>' /orders
Pw-Render: navigation
```

**What it costs.** Your own rule is that a response has to claim the mode it is, because that is where a proxy substitution is detected. A client that enforces it discards every tree it fetches. Ours then falls back per operation, and since a fragment that arrived as values carries no markup to fall back to, each one fails and the navigation becomes a complete document. The only trace is that pages got bigger.

**We corrected it on the answer**, because the header is ours to write now.

**The ask:** a sequence case in `modeName`, or the literal set on the response the way `action` still does it.

> **v0.4.8:** the `default` arm is gone and the switch is exhaustive — a mode with no name panics. Unreachable while `Negotiate` resolves the unknown to a document, which is the point: the next mode added fails at the first test instead of shipping a response that lies. Our correction is deleted.

## 2. A hole inside a table leaves the table

The placeholder is an unknown element:

```go
// htmlbind/delta/boundary.go:229
return `<` + c.element + ` ` + attr + `="` + id + `" style="display:contents"></` + c.element + `>`
```

An unknown element in table context is foster-parented: the HTML tree construction algorithm takes it out of the `tbody` and inserts it immediately before the `table`. This is the parser, not a browser quirk, and it is not avoidable by the caller.

```js
const t = document.createElement("template");
t.innerHTML = '<table><tbody><tb-boundary data-tb-id="row-1"></tb-boundary></tbody></table>';
t.innerHTML
// '<tb-boundary data-tb-id="row-1"></tb-boundary><table><tbody></tbody></table>'
```

**What it costs.** Every hole a table's rows leave sits outside the table. The rows that fill them land loose on the page and the list is left empty. The response was correct and the DOM is valid, so nothing reports it — we found it by counting rows after a navigation.

**It is wider than the delta.** A progressive render writes the same element for an await boundary (`htmlbind/ops.go:432`, `:747`), so a streamed document has the same problem — and there it separates the wrapper from its own fallback:

```js
new DOMParser().parseFromString(
  '<table><tbody><tb-boundary id="tb-1"><tr><td>loading…</td></tr></tb-boundary></tbody></table>',
  "text/html").body.innerHTML
// '<tb-boundary id="tb-1"></tb-boundary><table><tbody><tr><td>loading…</td></tr></tbody></table>'
```

The placeholder is outside the table and its fallback row is inside it. A client that settles a boundary by id then writes the finished row where the placeholder ended up — outside the table — and the fallback row stays in the list permanently. We verified the parse; the consequence follows from replacing by id, which is what every client does.

**We rewrite holes to `<template>` before parsing and restore the spelling afterwards through the DOM**, where insertion is not parsing and nothing is foster-parented. It works, and it is a rewrite of your markup in our client, which is the wrong place for it.

**The ask:** a placeholder the parser keeps where it was written. Two candidates, both verified above:

| | in table context | carries attributes | renders |
| --- | --- | --- | --- |
| `<template data-tb-id="…">` | kept | yes | nothing |
| `<!--tb-hole:…-->` | kept | no | nothing |

`<template>` is the smaller change: it stays an element, so `querySelector` and every existing lookup are unaffected, and it needs no `display:contents` because a template never renders. A comment is cheaper on the wire and would need the client to walk siblings instead of querying. Either would also fix the streamed await case, which today has no workaround at all — a caller cannot rewrite a document the browser is parsing as it arrives.

> **v0.4.8:** both, and the reason they had to split is one we missed. A `<template>` keeps its place in a table but does not render its contents, and a fallback that does not render is not a fallback. So the delta hole is a `<template>` and the await marker is a comment fence around the fallback — one node to replace, and one range around visible content, which no single shape can be. Our rewrite-before-parse is deleted; settling now walks to the fence.

## 3. Still open from v0.4.6: `valuesAreSmaller` is not applied on the streamed path

Unchanged in v0.4.7:

```go
// htmlupdate.go:248 — buffered, and the redraw
if sequences && operation.Sequence != "" && valuesAreSmaller(operation) { … }

// stream.go:369 — streamed
if sequences && item.Operation.Sequence != "" { … }
```

So "the split is never a loss, because the choice is made per fragment" holds on the buffered path and not on its sibling — and the streamed path is what every navigation we serve goes through. Your own comment names the shape that inverts: *a fragment of two elements costs more as an address plus its values than as the markup itself; a list row is exactly that shape.*

We are still reporting the asymmetry rather than a number. It does not bite the page we measured, where the changed fragment is large.

This is the same shape as the `children` dispatch before it: a rule applied on one path and not on its sibling.

> **v0.4.8:** one predicate both paths call, so the rule cannot be on one and not the other again.

## One place we read the guide and chose differently

The migration note asks for `RedrawHeaders` before the branch, so a page response declares the redraw axes too. We declare `Pw-Render` and `Pw-Build` before the branch and leave `Pw-Kind` and `Pw-Instance` to the redraw response itself.

The reasoning: every update request names its mode on the render header and a document names none, so a page response varying on `Pw-Render` already cannot be handed to a redraw request — the stored response's own Vary is what a cache matches on, and `Pw-Render` mismatches. Kind and instance separate one redraw from another, and every redraw response carries both. A document varying on headers it never reads fragments a cache for nothing.

If there is a case we are missing — a cache that matches on a union rather than on each stored response's own Vary — we would rather be told than be right by accident.

> **v0.4.8:** the reading holds and the guide's reason did not. But the placement was carrying something neither side had noticed: `FailureResponse` had no `Vary` at all, so a heuristically cacheable 404 could answer a document request from the same URL, and declaring the shared axes before the branch was covering it. Refusals now carry the negotiated mode's axes. Right answer, wrong reason, and the advice outlived its argument.

## What did not move, and one that cannot

Everything else in the migration note applied cleanly. The client obligations in section B were already met from the v0.4.4–v0.4.6 rounds, and section C's sequence walk has been in since v0.4.6.

The one header we could not take is the **redraw ETag**, which digests the body you assemble. A caller cannot produce it without rendering the component a second time, so it stays yours — which is the correct exception to the rule and worth stating in the guide beside the rule.

## How to reproduce both

`examples/partial_update` in this repository, with `html.update.enabled` on: a layout, a page with a search form, a `@reloadable` table, and a `@reloadable` row inside it. Load it and click a link. On an unmodified v0.4.7 every sequence tree is discarded (1), and the table comes back empty with its rows loose in the page (2).
