# Measurements: what a navigation costs

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** `github.com/shibukawa/tinybind-go` v0.4.6
**Answers:** "Transfer size per delivery, before and after, on real pages. That number should decide whether the deferred step above gets built."

## Summary

Two results, one of which we did not expect.

**Sending values instead of markup is worth 3.3× on a content-heavy page**, and 2.4× against the complete document it replaces. The static trees cost about a kilobyte, once per build, cached immutably.

**A markup delta is larger than the document it replaces.** Not marginally: 13,614 bytes against 9,787. The delta is correct and minimal — it transfers one region and restates the layout as an unchanged validator in 89 bytes — and it still loses, because a record escapes every angle bracket of the markup it carries. On this page the same fragment is 9,295 bytes as HTML and 13,001 bytes on the wire.

That second result is the argument for the split, and it is stronger than the ratio. A partial update that transfers less of the page and more bytes than a full page load is a hard thing to defend on its own.

## The numbers

A page of 25 result rows under a shared layout: a document shell, a layout carrying a nav, and a page carrying a search form and a table. Classes and attributes are on the markup because that is what a real page carries.

| | bytes | vs document | |
| --- | ---: | ---: | --- |
| complete document | 9,787 | 1.00× | what a browser with no runtime receives |
| cross-page link, markup | 13,614 | 0.72× | fragment 13,001 B on the wire, 9,295 B as HTML — 1.40× |
| cross-page link, values | 4,078 | 2.40× | 3.34× smaller than the same delta as markup |
| cross-page link, trees | 1,003 | — | 1 tree, fetched once per build |
| same-page search, markup | 13,692 | 0.71× | fragment 13,079 B on the wire, 9,373 B as HTML — 1.40× |
| same-page search, values | 4,156 | 2.35× | 3.29× smaller than the same delta as markup |
| same-page search, trees | 1,003 | — | 1 tree, fetched once per build |

Both scenarios are the ordinary ones: an `<a>` to a sibling route under the same layout, and a GET search form re-rendering the page it is on.

**Method.** Measured through `pw.WriteHTMLChain`, which is the entry this framework serves every page from, so the bytes are the response and not a reconstruction of it. The client's manifest is built by parsing a previous response's operation records exactly as our runtime does, and returned on the next request. `pw/transfermeasure_test.go` is the harness; `go test ./pw -run TransferCost -v` reprints the table.

## What the records look like

The delta is not doing anything wasteful. Cross-page, warm client:

```
   492  {"r":"head","head":["…the runtime meta tag…"]}
 13001  {"r":"op","kind":"replace","id":"c2","html":"…the results section…"}
    89  {"r":"op","id":"c1","frame":"lL0ncQ…","children":"QA52cO…"}
    28  {"r":"end","reason":"final"}
```

The layout is recognised as unchanged and costs 89 bytes. The results section is genuinely all that changed. The 13,001 bytes are the encoding, not the selection.

## Three things this changes about how we read the design

**The escaping multiplier is 1.40× on real markup, not 3×.** We reported 3× earlier from `<p>one</p>`, which is almost entirely angle brackets. A real fragment is mostly attribute values, class names, and text, so the true figure is lower — and still enough to lose to the document.

**The break-even is a page-shape property, not a size property.** A delta wins when the changed region is a small part of the page and loses when it is most of it. A dashboard where one panel updates wins; a search result list, which is the case a delta is most obviously for, loses. We would not have predicted that, and neither the ratio nor the record count shows it.

**The cold client pays twice.** A page load leaves our runtime holding no validators, so the first in-page navigation after arriving sends no manifest and is answered with a complete document. Only the second click onward is a delta. That is a property of our runtime rather than of the module, and we mention it because it halves the number of navigations any of these figures apply to. Seeding the manifest from the document — as we already do for the live stream, on the terminal marker — would fix it, and we will measure whether the seed costs less than the navigation it saves before deciding.

## Our reading

The split is worth building, and it was worth building for a reason other than the one we argued. We asked for it to make deltas smaller. The measurement says it is what makes deltas *smaller than not having them* on the pages where partial updates are most obviously wanted.

We are not asking for anything here. The deferred step in `httpbind_render_modes.md` — applying a value to a node without reparsing — is a separate question, and this measurement says nothing about it: every figure above is transfer, and that step is about what the browser does after the bytes arrive. If it is worth measuring, the thing to measure is time to a stable frame rather than bytes, and we would need a browser harness we do not have yet.
