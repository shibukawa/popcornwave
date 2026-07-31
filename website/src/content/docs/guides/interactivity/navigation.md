---
title: Navigation
description: Making ordinary page-to-page navigation feel continuous and instant — view transitions, speculation rules, and when not to navigate at all.
sidebar:
  order: 3
---

The reason single-page applications took over navigation was never routing. It
was the two things a full page load used to cost: a white flash between
documents, and a wait long enough to notice.

Both now have platform answers that a server-rendered application can adopt
without touching a handler. Neither changes what a route returns, and both
degrade to exactly what you have today.

## Continuity between pages

Add one rule to a stylesheet both pages load:

```css
@view-transition { navigation: auto }
```

Same-origin navigations now cross-fade instead of cutting. That is the whole
setup, and it costs one declaration.

The rule passes through a component's scoped `<head>` block untouched — it is
an at-rule with no selector to scope — but the natural home is the shared
stylesheet or the document shell, since a transition is a property of the site
rather than of one component.

Naming an element makes it travel rather than fade. Give the same
`view-transition-name` to the thing that is conceptually the same on both
pages — a thumbnail on the list and the header image on the detail page:

```html
<head>
<style>
.hero { view-transition-name: task-hero }
@media (prefers-reduced-motion: reduce) {
  :global(::view-transition-group(*)) { animation: none }
}
</style>
</head>
```

A `view-transition-name` must be unique in the document at the moment the
transition starts. In a list, that means the name belongs to the item being
navigated to, not to every row — so put it on the row the click targeted, or
name rows individually.

Cross-document view transitions are not in every engine yet. Where they are
missing the navigation happens exactly as before, which is why this is safe to
add early.

## Navigation that has already happened

Speculation rules tell the browser to fetch — or fully render — a page before
the click. They belong to the document shell, and the JSON goes in as it stands:

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head>
  <meta charset="utf-8">
  <title>My App</title>
  <script type="speculationrules">
  {"prerender": [{"where": {"href_matches": "/tasks/*"}, "eagerness": "moderate"}]}
  </script>
</head>
<body><slot /></body></html>
}
```

Braces in script content are literal unless they read as a template insertion,
which a rules block never does — see
[Browser controls](/guides/interactivity/browser-controls/) for the case that does.

The alternative avoids the inline script altogether — point at the rules with a
response header, which is what a strict `script-src` policy prefers:

```
Speculation-Rules: "/speculation-rules"
```

That route has to answer with `application/speculationrules+json`, so it is a
small handler rather than a file dropped into `public/`, where the media type
comes from the extension.

| Eagerness | Triggers on | Use for |
| --- | --- | --- |
| `conservative` | pointer or touch down | anything expensive to produce |
| `moderate` | hover of about 200 ms | ordinary detail pages |
| `eager` | as soon as the rule is seen | a small, known set of likely next pages |

`prefetch` instead of `prerender` fetches the document without rendering it; it
is cheaper, weaker, and a reasonable default for a large site.

:::caution
A prerender **runs the page**: the request is issued and any script on it
executes, before the reader has decided to go there. Restrict the rules to
routes that are safe to request twice, and never let one match a GET that
changes state.

Classic Popcorn Wave applications are usually fine here, because writes go
through POST and Post/Redirect/Get. The pattern to check for is a link that
performs an action — `GET /logout`, `GET /items/5/archive`. Those should be
forms regardless; a speculation rule turns the latent bug into a live one.
:::

Two more things worth knowing before you widen the `href_matches` pattern:

- A prerendered page that is never visited still cost the server a render. Match
  the routes readers actually go to next, not everything.
- Analytics that count a server-side render as a visit will over-count.
  `document.prerendering` and the `prerenderingchange` event let client-side
  measurement wait until activation.

Speculation rules are a Chromium feature today. Elsewhere the block is ignored.

## Not navigating at all

A fragment swap replaces a region without a navigation, which means no URL
change, no history entry, and nothing for the back button to return to. That is
either exactly right or quietly wrong, depending on the state involved.

| The state is | Do | Because |
| --- | --- | --- |
| worth sharing, bookmarking, or returning to | navigate | a URL is the only thing that carries it |
| a filter the reader will refine ten times | swap | ten history entries is ten wrong answers for the back button |
| a step in a flow the reader may abandon | navigate | leaving must be possible |
| a detail panel beside a list | swap, and reflect it in the URL if it can be linked | both properties are wanted |

The classic answer is often the right one. A filter form that submits normally
gives you a shareable URL, a working back button, and a correct empty state for
free, and it costs one round trip that a prerender may already have paid.

When you do need the swap, the URL stays yours to manage: `history.pushState`,
or the equivalent attribute in whichever swap library the document shell loads.
The framework takes no part in it — it answers with markup and a status, and
nothing in a fragment response describes a URL.

## Scroll and Post/Redirect/Get

Answering a POST with `303 See Other` and a `Location` remains the correct shape
for a write, and it interacts well with everything above: the redirect target is
an ordinary GET, so it can be prerendered, transitioned into, and re-visited
without re-submitting.

Where a redirect lands the reader is a real decision. Sending them back to the
list with a fragment identifier for the row they just edited keeps the position
they had, without any scroll restoration code.
