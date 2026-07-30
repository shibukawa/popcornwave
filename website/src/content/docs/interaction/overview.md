---
title: Overview
description: Where interactivity comes from in a server-rendered application, and which tier of the ladder to reach for first.
sidebar:
  order: 1
---

Popcorn Wave renders on the server. It ships no hydration, no client-side
router, and no client state store, and it is not going to grow one quietly. So
the question a real application asks — *how does this button open a menu, this
row edit in place, this page not flash white* — is answered outside the
framework.

That is not a gap to be worked around. It is a choice with an order to it. Most
of what an interface needs is already in the browser, costs nothing, and keeps
working when a script fails to load. What is left is small enough to buy
deliberately.

## The ladder

| Tier | What you add | What it can do | What it cannot |
| --- | --- | --- | --- |
| **Browser** | nothing | show, hide, position, transition, and validate what the page already holds | know anything the server did not send |
| **CSS components** | one Tailwind plugin | give that markup an appearance and a theme | change behaviour |
| **Server fragments** | one swap library in the document shell | replace a region with markup only the server can produce | act without a round trip |
| **Authored islands** | your own JavaScript | local state and events no round trip can answer | come for free — you own it, including its failures |

The rule is to take the lowest tier that can express the interaction, and to
climb only when a tier below genuinely cannot. Familiarity is not a reason: a
dropdown built from a `popover` attribute and a dropdown built from a component
library look identical to the reader, and only one of them can break.

## What to reach for

| You want | Cheapest tier that works | Where |
| --- | --- | --- |
| Modal, confirmation | `<dialog>` | [Browser controls](/interaction/controls/) |
| Dropdown menu, tooltip | Popover API | [Browser controls](/interaction/controls/) |
| Accordion, disclosure | `<details name>` | [Browser controls](/interaction/controls/) |
| Toast, notification | `popover="manual"` | [Fragments and islands](/interaction/fragments/) |
| Input feedback before submit | constraint validation, `:user-invalid` | [Forms](/interaction/forms/) |
| Field errors after submit | the server re-renders the form | [Forms](/interaction/forms/) |
| Small suggestion list | `<datalist>` | [Forms](/interaction/forms/) |
| Page-to-page continuity | view transitions | [Navigation](/interaction/navigation/) |
| Navigation that feels instant | speculation rules | [Navigation](/interaction/navigation/) |
| Filter, inline edit, live list | fragment swap | [Fragments and islands](/interaction/fragments/) |
| Dialog whose contents come from the server | fragment swap into a `<dialog>` | [Fragments and islands](/interaction/fragments/) |
| Client-only state, drag, canvas | a custom element | [Fragments and islands](/interaction/fragments/) |
| Optimistic update, offline edit | — | not this framework |

## Appearance is a separate axis

Nothing above says what any of it looks like. Scoped component styles and
Tailwind are covered in [Styling](/guides/styling/), and daisyUI sits on top of
Tailwind as a set of **CSS-only** component classes plus `data-theme` theming.

The important property for this section is that daisyUI styles markup you wrote
rather than replacing it. A `<dialog>` keeps being a `<dialog>` — top layer,
`Esc`, focus handling and all — when it also carries `class="modal"`. Structure
and accessibility semantics stay yours, which is exactly what lets the browser
tier and the CSS tier be used at the same time rather than as alternatives.

## What the framework does contribute

Two things, both of which the pages that follow lean on:

- **`pw.WriteHTMLFragment`** renders one template and nothing else, so a region
  can be re-rendered by the same component that first drew it. See
  [Responses](/guides/responses/) for the contract and `examples/htmx_fragment`
  for a full application built on it.
- **Async rendering** streams a page whose slow parts arrive later, which
  removes a whole class of client-side loading state. See
  [Async rendering](/advanced/async-rendering/).

Everything else on this ladder is standard web platform work. The framework
does not name a swap library, does not wrap a CSS plugin, and no route knows
which client library called it.

## On browser support

Some features here are old enough to assume and some are not. Each page marks
the difference, and the marked ones are used the same way: as an enhancement
layered over something that already works without it. A view transition that
does not run leaves an ordinary navigation; a prerender that never happens
leaves an ordinary click.

:::note
Support statements on these pages were checked in **July 2026** and are the part
most likely to age. Confirm anything load-bearing against
[Baseline](https://web.dev/baseline/) before relying on it.
:::
