---
title: Partial Updates
description: Answer a page request with only the regions that changed, so a filter, a redraw, or a form submission costs the region rather than the whole document.
sidebar:
  order: 4
---

A search page is a function of its request. Change the sort order and the server
renders the layout, the navigation, the footer and the results — and the browser
throws away four of those five and paints the one that moved. It already had the
rest.

Partial updates close that gap. The same URL answers a complete document to
anything that asks for one, and to a page that already holds the layout it
answers only the regions whose markup actually changed.

Turn it on in `popcornwave.toml`:

```toml
[html]
[html.update]
enabled = true
validator_key = "${HTML_UPDATE_VALIDATOR_KEY}"
```

It is off by default because a page that reloads acceptably does not need this,
and the feature is not free: it adds a browser runtime to every document, a
secret to deploy, and a rule that every re-render must be free of side effects.
Reach for it when a screen is refreshed often enough that the reload is the
thing people notice.

## What the server sends, and who applies it

The difference is not that one response is smaller. It is that they are
different kinds of thing, and something different turns each into a page.

![Two flows through one handler. In blue, the browser requests the page, the handler renders the whole chain, and the browser paints a new page — the runtime is never involved. In green, the runtime intercepts a link, sends the same request with the render and manifest headers, the same handler compares each region against the digests it carried, answers with instructions to replace one region and nothing else, and the runtime swaps that region into the live DOM](../../../../assets/diagrams/partial-update-sequence.svg)

Read the blue path first. It is a request with no update header, and it is the
one every client can take: the response is a **document**, and the browser's own
parser is what replaces the page. No JavaScript takes part, which is why the
runtime lifeline is untouched.

The green path is the same URL and the same handler. What changed is the
headers, and what comes back is not a page but **instructions** — one region
named, its new markup beside it, and nothing said about the layout, the
navigation or the footer, because the digests for those matched what the request
already held.

That last part is what the runtime is for. It swaps the named region into the
DOM that is already on screen, so everything around it stays exactly as it was:
focus, scroll position, an open dropdown, half-typed text. A reparsed document
could preserve none of that.

A redraw is the same green path aimed at one component rather than a whole
route. The request names it with `Pw-Kind` and `Pw-Instance`, and the response
is that component's markup with no envelope at all — which is why the endpoint
stays readable with `curl`.

Every one of these falls back the same way. If the runtime is absent, if a proxy
stripped the header, or if anything goes wrong at any point, the request becomes
an ordinary navigation and the answer is the blue path.

## Nothing about the page changes

Every layout and page of a rendered chain is already an update boundary. A
`.pw.html` chain compiles to `_pw_gen.go` carrying an identity on each boundary
root and a digest of what it rendered, so a request that arrives holding the old
digests can be answered with the difference.

That means the handler is unchanged, the templates are unchanged, and a request
that asks for nothing gets the bytes it always got. A crawler, `curl`, and a
browser that never ran the script are unaffected by all of this.

An ordinary component is deliberately *not* a boundary. A five-hundred-row list
would otherwise put five hundred entries in every request.

## Three paths, and the rule that picks one

The three differ in who holds the input that changed.

**Navigation** owns inputs the server derives from the request — a search
parameter, a route. The runtime intercepts a same-origin link or a GET form,
re-requests the page's own URL, and the server sends back the boundaries whose
markup differs. Nothing is written on either side; this is what `enabled = true`
buys on its own.

**Redraw** owns inputs the browser holds, for a region whose state should not
appear in a shareable URL. The component is declared reloadable and re-rendered
alone, with no page execution.

**Action** owns a mutation. The handler performs it and answers the same request
with the regions it changed, so one round trip both acts and refreshes.

The rule for choosing is about the URL, not about the mechanism: **state that
can live in the URL belongs there.** A sort order, a page number, and a filter
all make the page shareable, bookmarkable and back-navigable, and navigation
handles them with no code. Reach for a redraw when putting the state in the URL
would be wrong — a widget's local expansion, a panel that polls. There is no
third option for "re-run the handler with one argument patched", because that
could not reach the data fetch that produced the component's other inputs.

## A reloadable component

Annotate the component and give it an id its caller writes:

```html
<!-- templates/card.pw.html -->
package templates

@reloadable
export component OrderCard(id: string, orderID: int): html {
<article class="card">
  <h3>Order {orderID}</h3>
</article>
}
```

Generation refuses anything it cannot serve from a URL. The component must be
exported and render exactly one root element, the `id` parameter is required,
and every other parameter must be a type a query string carries
deterministically — a record, a slice and `html` are errors rather than
warnings, because you asked for the endpoint.

Nothing else is written. Generation folds each component's call graph down to
the reloadable components its markup can contain and puts that set on the page's
own parameters, so a handler names the page and never a list that could fall out
of step with the template. A page under a page tree needs no handler code at
all — the render entry answers the redraw.

## Paying less for a redraw

A redraw answered inside the page render costs whatever the handler did to build
that page — the query behind the list, the fetch behind the header — none of
which the redraw needed. Answer it earlier to skip all of that:

```go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
	"myapp/templates"
)

func Orders(w http.ResponseWriter, r *http.Request) {
	if !pw.Authenticated(r.Context()) {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	if pw.Redraw(w, r, templates.OrdersPage) {
		return
	}
	orders, err := loadOrders(r.Context())
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteHTMLPage(w, r, nil, templates.OrdersPage(templates.OrdersPageParams{Orders: orders}))
}
```

The page is *named*, not called, so nothing builds its parameters and the data
behind them is never fetched.

The capability is the same either way. What the line buys is the query it jumps
over, and a narrower surface: the set comes from one page's markup rather than
from everything the deployment publishes, so this URL cannot be asked for a
component this page never shows. A page whose markup reaches no reloadable
component does not compile here at all, which is the honest answer — there is
nothing on it to redraw.

Where it sits matters as much as that it is there. Above the data load, so the
redraw skips it; below the authorization check, so a request this handler would
refuse never reaches a component.

Both forms are answered **at the page's own URL**, which is what makes the redraw
inherit whatever guards the page. A reserved path would have needed a second
protection rule kept in step with the first, and nothing forces two such rules to
agree.

## Answering a mutation

Here is the handler before any of this. It renames, then redirects, which is
post-redirect-get and is what a form submission has always done:

```go
func Rename(w http.ResponseWriter, r *http.Request) {
	order, err := renameOrder(r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/orders", http.StatusSeeOther)
}
```

The update version adds a branch and changes nothing above it:

```go
func Rename(w http.ResponseWriter, r *http.Request) {
	order, err := renameOrder(r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}

	// Everything above is unchanged, and everything below is the update path.
	// A client that cannot apply regions never reaches it.
	if !pw.WantsUpdate(r) {
		http.Redirect(w, r, "/orders", http.StatusSeeOther)
		return
	}
	pw.WriteUpdate(w, r, http.StatusOK,
		pw.Replace("order-summary", templates.Summary(templates.SummaryParams{Order: order})))
}
```

The mutation stays where it was, above the branch, so it runs exactly once for
either kind of client. One predicate is what keeps the two paths from drifting:
an ordinary form submission and a client without the runtime take the redirect,
and a page that can apply regions takes the regions.

The status is the handler's own, and the browser applies the regions whatever it
says. A rejected submission returns 4xx and the regions it carries *are* the
validation errors — showing them is the point. That is the opposite of a redraw,
where a non-2xx means the render failed and the page reloads.

When the action changed where the user belongs, say so rather than guessing which
regions to rewrite: `pw.WriteUpdateNavigate(w, r, "/orders/17")`.

## From the browser

The runtime installs one namespaced object, and an author feature-detects it
because a page may load with updates disabled:

```js
if (window.popcornwave) {
	await window.popcornwave.update({ sort: "newest" });      // this route, new parameters
	await window.popcornwave.redraw("card-17", { orderID: 17 });
	const response = await fetch("/orders/rename", {
		method: "POST",
		headers: window.popcornwave.updateHeaders(),
		body: form,
	});
	await window.popcornwave.apply(response);
}
```

Links and GET forms are intercepted by default, so a search form refines the page
it is on with no script at all. Put `data-tb-ignore` on an element or an ancestor
to hand one back to the browser. Non-GET submissions, modified clicks, `target`,
`download` and cross-origin URLs are always the browser's, which is what keeps
post-redirect-get working exactly as it did.

A region the server does not own — a map widget, a canvas, a video mid-playback —
is marked `data-tb-preserve="chart"` and moved into the replacement rather than
re-rendered.

## What will bite you

**A missing validator key fails startup.** The digests are keyed, because an
unkeyed digest of low-entropy content lets somebody confirm a guess by comparing
digests. Startup refuses `enabled = true` with no key rather than serving unkeyed
ones. Rotating the key is not a break — comparisons miss and the next response is
a complete document.

**A re-render may be discarded after the server produced it.** A superseded
response is dropped unapplied, so rendering must be free of side effects.
Mutations belong in an action response.

**A redraw's arguments come from whoever asked.** Everything but the instance id
arrives from the caller, so a component that loads a record by identifier must
check ownership itself, exactly as a handler does. Naming components in
`pw.Redraw` bounds *which* components a URL will answer for; it does not vouch
for their arguments.

**A boundary that embeds the clock never matches.** A region rendering
"updated 3 seconds ago" differs on every render and is re-sent every time. Push
that into a live boundary or into the browser.

**A GET update cannot clear a form back to its default.** The markup is
identical, so nothing tells the runtime to discard what the user typed — which is
the same rule that protects their typing everywhere else. Post-redirect-get
clears through an ordinary page load.

## When to stay away

Use [Fragments and islands](/guides/interactivity/fragments/) instead when the
application wants to own the swapping: `pw.WriteHTMLFragment` renders one
template with no negotiation, no boundary identity and no ordering guarantee, and
a swap library decides what happens to it. That is the right shape for a dialog
whose contents come from a route the application chose.

Use [Live Rendering](/guides/cross-layer/live-rendering/) when the *server* is
what learns something new. Partial updates answer a request; a live boundary
keeps delivering without one.

And do not reach for either on a page that reloads in a hundred milliseconds. The
document path is the one every client can take.

Every key, with its default, is in [Configuration](/reference/configuration/).
