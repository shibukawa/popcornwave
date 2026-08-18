# Measurements: what a navigation costs

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.4.6
**Answers:** "Transfer size per delivery, before and after, on real pages. That number should decide whether the deferred step above gets built."

## What is being compared

Three numbers, and only one of them is what this runtime receives today. Saying which is which turns out to matter, because reading the middle one as a live cost produces a claim about a configuration nobody runs.

| | who receives it |
| --- | --- |
| the complete document | a browser with no runtime, a crawler, `curl` |
| the delta **as markup** | a client that does not walk sequences — and this runtime, until the walk landed a few days ago |
| the delta **as values** | **this runtime now** |

Our client sets the sequences header on every request it issues, and the streamed path answers with values wherever a fragment has a sequence address — unconditionally, not by size. So on a navigation we receive values, and the markup row is the *before* of before-and-after rather than a cost anyone is paying.

## The numbers

A page of 25 result rows under a shared layout: a document shell, a layout carrying a nav, and a page carrying a search form and a table. Classes and attributes are on the markup because that is what a real page carries.

| | bytes | vs document | |
| --- | ---: | ---: | --- |
| complete document | 9,787 | 1.00× | |
| cross-page link, as markup | 13,614 | 0.72× | baseline; fragment 13,001 B on the wire, 9,295 B as HTML — 1.40× |
| **cross-page link, as values** | **4,078** | **2.40×** | **what we receive**; 3.34× the markup baseline |
| cross-page link, trees | 1,003 | — | 1 tree, fetched once per build, `immutable` |
| same-page search, as markup | 13,692 | 0.71× | baseline; fragment 13,079 B on the wire, 9,373 B as HTML — 1.40× |
| **same-page search, as values** | **4,156** | **2.35×** | **what we receive**; 3.29× the markup baseline |
| same-page search, trees | 1,003 | — | 1 tree, fetched once per build, `immutable` |

Both scenarios are the ordinary ones: an `<a>` to a sibling route under the same layout, and a GET search form re-rendering the page it is on. They produce the same record shape, which is the point — one mode, one grammar, and only which boundaries compare equal differs.

**Method.** Measured through `pw.WriteHTMLChain`, the entry this framework serves every page from, so the bytes are the response and not a reconstruction of it. The client's manifest is rebuilt from a previous response's operation records exactly as our runtime does, and returned on the next request. `pw/transfermeasure_test.go` is the harness; `go test ./pw -run TransferCost -v` reprints the table.

## What actually goes on the wire

Cross-page, warm client, as we receive it:

```
  492 B  {"r":"head","head":["…the runtime meta tag…"]}
 3465 B  {"r":"op","kind":"replace","id":"c2","frame":"kb71Io…","parent":"c1",
          "seq":"d-AOnskBnIZ-Rp7hVF9Keg","values":[" data-tb-id=\"c2\""," value=\"\"","25", …]}
   89 B  {"r":"op","id":"c1","frame":"lJRwkm…","children":"lLBlyx…"}
   28 B  {"r":"end","reason":"final"}
```

The layout is recognised unchanged and costs 89 bytes of restated validators. The results section carries no markup at all. The same-page search differs only in the values — the search field's `value="two"` — and reuses the identical `seq`, so no tree is fetched a second time.

## The result

**Values are worth 3.3× against the same delta as markup, and 2.4× against the complete document**, on both scenarios, with about a kilobyte of trees fetched once per build and cached immutably.

The second figure is the one that decides the question. A delta is only worth its complexity if it beats the document it replaces — and **as markup, on this page, it does not**: 13,614 bytes against 9,787. The delta is correct and minimal while that happens; it transfers one region and restates the layout in 89 bytes. It loses because a record escapes every angle bracket of the markup it carries, so the same fragment is 9,295 bytes as HTML and 13,001 on the wire.

That is a statement about the baseline, not about anything shipping. Its force is that it is **the reason to build the split**, not a defect to fix: without sequences, a partial update on the pages partial updates are most obviously for transfers less of the page and more bytes than a full page load.

## Two corrections to what we have been saying

**The escaping multiplier is 1.40× on real markup, not 3×.** We reported 3× earlier from `<p>one</p>`, which is almost entirely angle brackets. A real fragment is mostly attribute values, class names, and text.

**The break-even is a page shape rather than a size.** A delta wins where the changed region is a small part of the page and loses where it is most of it. A dashboard panel wins; a search result list — the case a delta is most obviously for — loses. Neither the ratio nor the record count shows that.

## One property of our runtime the measurement surfaced

**The cold client pays twice, and this one is current.** A page load leaves our runtime holding no validators, so the first in-page navigation after arriving sends no manifest and is answered with a complete document. Only the second click onward is a delta. That halves the number of navigations every figure above applies to.

It is ours to fix rather than yours: we already seed the live stream's manifest from the document's terminal marker, and the same trick applies here. Whether the seed costs less than the navigation it saves is worth measuring before changing, and we have not.

## One thing we noticed while checking which half we receive

`valuesAreSmaller` gates the choice in `operationBody` — the buffered path and the redraw — and the streamed path does not call it:

```go
// operationBody
if sequences && operation.Sequence != "" && valuesAreSmaller(operation) { … }

// renderStream
if sequences && item.Operation.Sequence != "" { … }
```

So the guide's "the split is never a loss, because the choice is made per fragment" holds on the buffered path and not on the streamed one — and the streamed one is what every navigation we serve goes through. Your own comment names the shape that inverts: *a fragment of two elements costs more as an address plus its values than as the markup itself; a list row is exactly that shape*.

It does not bite the page above, where the changed fragment is large. It would on a page whose row components are boundaries. We have not measured that yet and are reporting the asymmetry rather than a number — it is the same shape as the `children` dispatch: a rule applied on one path and not its sibling.

## What this says nothing about

Every figure here is transfer. The deferred step in `httpbind_render_modes.md` — applying a value to a node without reparsing — is about what the browser does after the bytes arrive, and nothing above measures it. If it is worth measuring, the thing to measure is time to a stable frame, and that needs a browser harness we do not have.
