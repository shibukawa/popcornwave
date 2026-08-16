---
title: Overview
description: Browser features rather than framework API — where interactivity comes from in a server-rendered application, and which tier of the ladder to reach for first.
sidebar:
  order: 1
---

Popcorn Wave renders on the server. It includes no hydration layer,
client-side router, or client state store, so interactions such as opening a
menu, editing a row in place, or smoothing a page transition use browser APIs
or an application-chosen library.

Start with the browser when it already provides the behavior. Native controls
add no framework runtime and continue to work when unrelated scripts fail.
Add client-side code only for the interactions that remain; if most of the
application depends on long-lived client state, a server-rendered architecture
may not be the right fit.

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
| Modal, confirmation | `<dialog>` | [Browser controls](/guides/interactivity/browser-controls/) |
| Dropdown menu, tooltip | Popover API | [Browser controls](/guides/interactivity/browser-controls/) |
| Accordion, disclosure | `<details name>` | [Browser controls](/guides/interactivity/browser-controls/) |
| Toast, notification | `popover="manual"` | [Fragments and islands](/guides/interactivity/fragments/) |
| Input feedback before submit | constraint validation, `:user-invalid` | [Forms](/guides/interactivity/forms/) |
| Field errors after submit | the server re-renders the form | [Forms](/guides/interactivity/forms/) |
| Small suggestion list | `<datalist>` | [Forms](/guides/interactivity/forms/) |
| Page-to-page continuity | view transitions | [Navigation](/guides/interactivity/navigation/) |
| Navigation that feels instant | speculation rules | [Navigation](/guides/interactivity/navigation/) |
| Filter, inline edit, live list | fragment swap | [Fragments and islands](/guides/interactivity/fragments/) |
| Declarative server-fragment swaps | htmx | [Integrating htmx](/guides/interactivity/htmx/) |
| Dialog whose contents come from the server | fragment swap into a `<dialog>` | [Fragments and islands](/guides/interactivity/fragments/) |
| Region that is slow to produce | `await` boundary | [Async rendering](/guides/cross-layer/async-rendering/) |
| Region that changes with nobody watching | live boundary | [Live rendering](/guides/cross-layer/live-rendering/) |
| Client-only state, drag, canvas | a custom element | [Fragments and islands](/guides/interactivity/fragments/) |
| Instance-local JavaScript with automatic teardown | a component script | [Component scripts](/guides/interactivity/component-scripts/) |
| Component-local client state | a React root inside one island | [Integrating React](/guides/interactivity/react/) |
| Optimistic update, offline edit | — | not this framework |

## Who moves first

Every row in that table begins with the reader: a click, a keystroke, a hover.
Two things an interface needs do not begin there, and they are the two places
where this framework has more to offer than a client-rendered one rather than
less.

The first is a region the server cannot produce quickly. The reader has already
asked for the page; the page is what is late. A component framework answers this
with a loading state per region — client state to hold it, a fetch to fill it, a
spinner you wrote, and a waterfall if one fetch depends on another. An `await`
boundary answers it with a `fallback` written beside the content it stands in
for. The shell and every fallback commit immediately, each slow region replaces
itself over the same response as its data settles, and the slow dependencies
behind them overlap instead of queueing. No client state describes any of it.
A crawler or a CLI client, which runs nothing, is served the settled document
instead — so what gets indexed is the page rather than the word "loading." A
browser with scripting turned off is the one case that keeps the fallbacks,
which is the ordinary reason to write a fallback worth reading.

The second is a value that changes while nobody is touching the page: a queue
depth, a build that finishes, a message somebody else sent. The instinct here is
`hx-trigger="every 2s"`, and it works — at the price of asking a question whose
answer is usually "nothing changed." Every open tab pays that price on its own
timer, and the interval is a guess between two failures: too short wastes
requests, too long leaves the number stale. A live boundary inverts the
direction. A Go source yields when it has something to say, the server re-renders
that one region, and the connection carrying it is already open. The template is
the same `await` clause; the browser applies the delivery through the runtime it
already loads. A reader with no script still sees one real render of the region,
because the document commits the source's first value before it ends.

| What moved | Who asked | What you write | Where |
| --- | --- | --- | --- |
| The reader | the reader | a swap, or nothing at all | this section |
| The page is slow to render | the reader, once | `async` parameter, `await`, `fallback` | [Async rendering](/guides/cross-layer/async-rendering/) |
| The data, on its own | nobody | `external live` source, same `await` clause | [Live rendering](/guides/cross-layer/live-rendering/) |

Neither is free, and their costs land somewhere a frontend developer does not
usually look. An `await` boundary gives up `Content-Length`, and its compression
flushes per boundary rather than once. A live boundary costs one open connection
per screen and one page execution per reconnect — server load rather than client
complexity, and a dashboard watched by two hundred people is two hundred renders
per tick rather than two hundred polls every two seconds.

One rule matters more than the rest when you design with them. A live region is
replaced on the server's clock, while the reader is doing something else, so it
renders output and never input: a `form`, `input`, `textarea`, or `select` inside
a live clause is a generation error rather than a runtime surprise. The form
stays outside the boundary and the changing data goes inside it — which is the
same split that keeps focus and selection intact, and the reason this is a
compiler rule instead of advice.

## Appearance is a separate axis

Nothing above says what any of it looks like. Scoped component styles and
Tailwind are covered in [Styling](/guides/frontend/styling/), and daisyUI sits on top of
Tailwind as a set of **CSS-only** component classes plus `data-theme` theming.

The important property for this section is that daisyUI styles markup you wrote
rather than replacing it. A `<dialog>` keeps being a `<dialog>` — top layer,
`Esc`, focus handling and all — when it also carries `class="modal"`. Structure
and accessibility semantics stay yours, which is exactly what lets the browser
tier and the CSS tier be used at the same time rather than as alternatives.

## What the framework does contribute

Five things, all of which the pages that follow lean on:

- **`pw.WriteHTMLFragment`** renders one template and nothing else, so a region
  can be re-rendered by the same component that first drew it. See
  [Responses](/guides/frontend/responses/) for the contract and `examples/htmx_fragment`
  for a full application built on it.
- **Async rendering** streams a page whose slow parts arrive later, which
  removes a whole class of client-side loading state. See
  [Async rendering](/guides/cross-layer/async-rendering/).
- **Live rendering** keeps re-rendering one region for as long as the reader
  holds the page open, which removes the polling that would otherwise stand in
  for it. See [Live rendering](/guides/cross-layer/live-rendering/).
- **A checked name for a browser handler**: a component's script block returns
  the functions its markup may call, and `on-click="increment"` names one on the
  element that triggers it, so a rename fails generation instead of a click
  doing nothing. See
  [Component scripts](/guides/interactivity/component-scripts/#handlers-the-markup-can-name).
- **A checked address for a mutation**, inside a page tree: `server-action`
  resolves the name of a Go handler to a generated endpoint, so a renamed
  function fails generation instead of a click failing in production. On a form
  it also submits without any runtime at all. A script wanting an answer rather
  than a response declares an ordinary Go function instead and calls it as
  `await actions.getUser({ id })`, typed at both ends, which removes the
  hand-written `fetch` and the decoding around it. See
  [Server actions](/guides/interactivity/server-actions/).

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
