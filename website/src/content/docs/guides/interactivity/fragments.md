---
title: Fragments and islands
description: Combining server-rendered fragments with dialogs, popovers and custom elements — and the framework rules that decide how they fit together.
sidebar:
  order: 5
---

:::note[Where the line falls]
The fragment surface on this page is the framework's. The swap library, the
popover, the custom element and the browser APIs they call are not — they are
what the application chooses and owns, including their failures.
:::

Browser-only techniques can rearrange markup the page already holds. A fragment
request is useful when the server must produce new markup: a database-filtered
list, a panel whose contents depend on a row, or a form that returns validation
errors without closing its dialog.

The API is one call. `pw.WriteHTMLFragment` renders one template without a
document shell, merged head, or framing. [Responses](/guides/frontend/responses/)
defines that contract, and `examples/htmx_fragment` contains a complete
application built on it.

The application still decides how the returned markup enters the current
document. The recipes below show that boundary with a `<dialog>`; for simple
show-and-hide behavior over existing markup, a browser control is cheaper than
a fragment request.

## Three rules that shape every recipe

All three follow from the missing document, and they decide more of the design
than the swap library does.

| Rule | Consequence |
| --- | --- |
| A fragment cannot contribute to the head | styles and scripts for a swapped region belong to the page that is already loaded |
| A fragment never streams | an `await` boundary settles on the server, and the `fallback` never reaches the browser |
| A fragment carries no envelope | the framework cannot tell a stale swap from a current one; ordering is the swap library's problem |

The first is enforced: a template with a `<head>` block answers 500 rather than
dropping the contribution. That failure is deliberate, and it is the single
most common surprise when moving a component from the page path to a swap.

## A dialog the server fills

The dialog element lives in the page. Only its contents are fetched:

```html
<dialog id="drawer" class="drawer">
  <div id="drawer-body"></div>
  <form method="dialog"><button value="close">Close</button></form>
</dialog>
```

```html
{for task in tasks}
  <li>
    <span>{task.title}</span>
    <button type="button" hx-get="/tasks/{task.id}/edit"
            hx-target="#drawer-body" hx-swap="innerHTML">Edit</button>
  </li>
{/for}
```

The route answers with the panel and nothing more:

```go
func editForm(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[editInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	task, ok := tasks.find(input.ID)
	if !ok {
		pw.WriteProblem(w, r, pw.NotFound("no such task"))
		return
	}
	pw.WriteHTMLFragment(w, r, EditForm(EditFormParams{Task: task}))
}
```

Opening it is the one line neither side provides. A delegated listener keeps it
out of every button:

```js
// public/drawer.js
document.body.addEventListener('htmx:afterSwap', (event) => {
  if (event.detail.target.id === 'drawer-body') {
    document.getElementById('drawer').showModal();
  }
});
```

Note what this arrangement avoids. The dialog is not swapped, so it does not
lose its open state; its styles live on the page, so the head rule is satisfied;
and `showModal()` on a dialog that is already modal does nothing, so a second
swap into an open drawer simply updates its contents.

The form inside the drawer then follows the fragment status contract: a rejected
submission comes back as HTML with a 200, targeting `#drawer-body`, and the
dialog stays open with the errors in it. A successful one can answer with the
updated row for the list and close the drawer from the same listener.

## A toast that survives the click

`popover="manual"` is the variant that does not light-dismiss, which is what a
notification needs. Put the element on the page and let a response fill it
out-of-band:

```html
<output id="toast" popover="manual" class="toast" aria-live="polite"></output>
```

One template can carry both the region being replaced and the out-of-band
element, because a fragment response is just markup:

```html
export component TaskList(tasks: Task[], note: string): html {
<ul id="task-list" class="task-list">
{for task in tasks}
  <li>{task.title}</li>
{/for}
</ul>
{if note != ''}
<output id="toast" popover="manual" class="toast" aria-live="polite" hx-swap-oob="true">{note}</output>
{/if}
}
```

```js
// public/toast.js
document.body.addEventListener('htmx:oobAfterSwap', () => {
  const toast = document.getElementById('toast');
  if (!toast || !toast.textContent.trim()) return;
  toast.showPopover();
  setTimeout(() => toast.hidePopover(), 4000);
});
```

The listener re-reads the element by id rather than keeping the one it was
handed, because an out-of-band swap **replaces** the matching element rather
than filling it. The same fact explains the response markup: the `popover`
attribute has to be repeated there, since the attribute belongs to the element
being substituted in, not to the hole it lands in.

## Waiting states

On the page path, an `await` boundary declares its own `fallback` and the
runtime replaces it when the value settles. On the fragment path that never
happens: the response is buffered, the boundary settles on the server, and the
body arrives finished. The `fallback` is not late — it is never sent.

So the waiting state for a swap belongs to the client:

```html
<button hx-get="/tasks/summary" hx-target="#summary" hx-indicator="#summary-spinner">
  Refresh
</button>
<span id="summary-spinner" class="spinner">counting…</span>
```

```html
<head>
<style>
.spinner { visibility: hidden }
:global(.htmx-request) .spinner, .spinner:global(.htmx-request) { visibility: visible }
</style>
</head>
```

The class the library toggles is not yours, so it needs `:global(...)` to
survive scoping. A pure-CSS alternative avoids the coupling entirely: give the
target a skeleton style and let the arriving markup replace it.

## One component, two call sites

The page and the swap must not be able to disagree about what a row looks like.
They will not if there is one definition:

```html
<TaskList tasks={tasks} emptyLabel={emptyLabel} />
```

`Home` calls it for the first paint; the partial routes call it alone for every
swap after that. Both call sites are type-checked against the same parameter
list, so a change that breaks one stops the build rather than producing two
renderings of the same data. This is the property that makes fragments cheap
here, and it is worth designing components around: the region a swap replaces
should be a component before it is a route.

## When the swap is a timer

Sooner or later a region has to change without anybody clicking anything. A swap
library will do it:

```html
<div id="queue" hx-get="/queue" hx-trigger="every 2s" hx-swap="innerHTML">…</div>
```

Two seconds is a guess, and it is wrong in both directions at once. Most of those
requests re-render a queue depth that did not move, and the ones that matter wait
up to two seconds to arrive. Shortening the interval multiplies the first
problem; lengthening it deepens the second. Meanwhile every open tab keeps its
own timer, so the load scales with tabs rather than with events, and the server
has no way to say "nothing happened, stop asking."

A live boundary removes the guess by letting the server speak when it has
something to say. The template is the `await` clause the previous section
already used, with a source that keeps producing instead of settling once:

```html
external live WatchQueue(): Depth

export component Queue(): html {
<section class="card" id="queue">
{await depth = WatchQueue()}
  <strong>{depth.waiting}</strong> waiting · <small>{depth.at}</small>
{fallback}
  <p class="pending">connecting…</p>
{/await}
</section>
}
```

```go
func WatchQueue(ctx context.Context) iter.Seq2[Depth, error] {
	return func(yield func(Depth, error) bool) {
		for event := range queue.Watch(ctx) {
			if !yield(Depth{Waiting: event.Waiting, At: event.At.Format("15:04:05")}, nil) {
				return
			}
		}
	}
}
```

No attribute on the markup, no interval, and no endpoint: the browser reconnects
to the page's own URL, and the runtime already in the document shell applies each
delivery. What arrives is the whole region rendered again, which is why the
source yields the current state rather than a diff — a reconnect after a dropped
connection needs the next value and nothing else.

Three consequences are worth knowing before you reach for it.

A live region renders output and never input. Placing a `form`, `input`,
`textarea`, or `select` inside a live clause fails generation, because a delivery
arriving while the reader types would discard what they typed with no warning.
Keep the form outside the boundary and the changing data inside it.

Announcement is yours to choose, and the two useful answers are opposite. A gauge
re-rendering every second should announce nothing; a message list should sit
inside `role="log"`, which implies polite. Put the attribute on an element
*around* the boundary — one inside the replaced subtree is destroyed and
recreated with the content, which resets the live region.

And a swap still wins whenever the reader is the one who moved. Filtering,
sorting, inline editing, and pagination are all faster to write and cheaper to
serve as a swap, because they need no open connection and no subscription. Live
delivery is for the case a swap cannot express: the value changed, and nobody on
this page did it.

[Live rendering](/guides/cross-layer/live-rendering/) covers the bounds, the
reconnect behaviour, and what a connection costs; `examples/live_render` is a
running dashboard built on it.

## Islands you write yourself

Some interactions have no server round trip in them at all — a copy button,
drag reordering, a canvas. That is the top of the ladder, and the natural
boundary is a custom element.

```html
<copy-button data-value={task.id} class="copy">
  <button type="button">Copy id</button>
</copy-button>
```

```js
// public/copy-button.js
class CopyButton extends HTMLElement {
  connectedCallback() {
    this.addEventListener('click', async () => {
      await navigator.clipboard.writeText(this.dataset.value);
      this.querySelector('button').textContent = 'Copied';
    });
  }
}
customElements.define('copy-button', CopyButton);
```

Three properties make this the right shape for a server-rendered application:

- **The server HTML is complete.** Without the script, the reader sees a button
  that does nothing rather than an empty box. Enhance markup that already means
  something.
- **Swapped fragments upgrade themselves.** An element inserted by a swap is
  upgraded and `connectedCallback` runs, so an island inside a re-rendered
  region needs no re-initialisation hook — which is exactly the bookkeeping that
  event handlers bound at page load get wrong.
- **It matches how the framework loads its own runtime.** The async rendering
  runtime is a module loaded by `src` from a revision-stamped path; keeping your
  islands in files under `public/` puts them under the same `script-src 'self'`
  policy. See [Async rendering](/guides/cross-layer/async-rendering/).

### Where an island posts

An island that changes something needs an address, and hardcoding
`/users/42/rename` in a template puts a string where a symbol belongs. Inside a
page tree, `server-action="Rename"` names the exported Go handler instead, and
generation lowers it to `data-tb-action="/_action/…/Rename"`. Rename the
function and generation fails at the template that referenced it, rather than
the click failing at runtime.

The framework runtime is what acts on that attribute. It intercepts the click,
posts to the address, applies whatever regions come back, and puts the current
`pw_csrf` cookie into the `X-CSRF-Token` header on the way — so an island that
fires an action wires none of that up itself. Where the island has to decide
before the mutation, leave `server-action` off the element and issue the request
yourself with `window.popcornweb.updateHeaders()` and
`window.popcornweb.apply()`, which carry the same token and apply the same
regions. See [Server actions](/guides/interactivity/server-actions/) and
[Integrating React](/guides/interactivity/react/#writing-back-to-the-server).

Prefer light DOM. A shadow root buys encapsulation you rarely need here and
costs you the page's stylesheet — Tailwind utilities and daisyUI classes on the
server-rendered children stop applying inside it.

Keep the state in the DOM the server produced. An island that builds its own
copy of the data has two copies, and the next swap replaces one of them.

## Loading the library at all

Whatever the document shell loads is what every page pays for. The options and
their costs are worked through in `examples/htmx_fragment`: a CDN URL pinned by
version and Subresource Integrity, or the same file vendored into `public/`,
which removes the third-party origin from both the network path and the
`script-src` policy.

Vendoring is the better default for an application that already serves its own
assets. `pw build` precompresses `public/` and the directory is embedded, so a
vendored library costs one committed file and no build configuration.

## Where this stops

The ladder has a top, and it is worth naming plainly. There is no hydration, no
client-side router, and no client state store, so a few things are not cheap
here at any tier:

- **Optimistic updates.** Every change shown to the reader has been to the
  server and back.
- **Offline or long-lived client state.** A form that must survive a closed
  laptop needs storage you write and reconcile yourself.
- **Highly stateful editors.** A spreadsheet, a diagram canvas, a realtime
  collaborative document. These are genuinely client applications, and the
  honest arrangement is to mount one on a page this framework serves rather than
  to assemble it out of swaps.

Everything short of that is well served by the four tiers, and most interfaces
never reach the top two.
