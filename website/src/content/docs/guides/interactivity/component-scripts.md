---
title: Component scripts
description: Give a component its own JavaScript with a setup that runs per instance and a teardown that runs when the region goes, instead of a script that evaluates once and never releases anything.
sidebar:
  order: 6
---

The [authored islands](/guides/interactivity/overview/) tier says you own the
JavaScript. Owning it used to mean a site-wide module in the document shell that
found its elements by selector, because a script tag in a page is evaluated once
per URL for the life of the document — and after a partial update replaced the
region it was watching, it had no way to know.

A component script fixes both halves. It lives beside the markup it belongs to,
its `setup` runs **per rendered instance**, and whatever it returns runs when
that instance goes away.

The block is part of a [template component](/guides/frontend/templates/), while
its lifecycle follows the DOM updates described in [Partial
updates](/guides/cross-layer/partial-updates/). For a complete dependency-heavy
example, [Integrating React](/guides/interactivity/react/) mounts and unmounts a
React root through this same hook.

```html
package shop

export component Countdown(deadline: string): html {
<script component>
  export function setup({ el, teardown }) {
    const label = el.querySelector("[data-remaining]");
    const timer = setInterval(() => {
      label.textContent = remaining(el.dataset.deadline);
    }, 1000);
    teardown(() => clearInterval(timer));
  }

  function remaining(iso) {
    const seconds = Math.max(0, (Date.parse(iso) - Date.now()) / 1000);
    return Math.floor(seconds) + "s";
  }
</script>
  <p class="countdown" data-deadline={deadline}>
    <span data-remaining>—</span>
  </p>
}
```

Render that component three times and `setup` runs three times, each with its
own element. Replace the region holding one of them and that one's interval is
cleared while the other two keep running.

## Where the block goes, and why it is marked

At the top of the declaration, beside the `head` block, before the markup. The
bare `component` attribute is required — without it, the element is an ordinary
`<script>` in your markup and keeps its current meaning, which matters because
`<script>{RawJavaScript(code)}</script>` is a real thing templates already do.
The marker is what selects the new reading, so adding this feature changed no
existing template.

Inside the block, content is read verbatim: a brace is JavaScript rather than an
interpolation, which is what makes a body script writable at all.

Three rules the generator enforces, each because the alternative fails quietly:

**One block per component.** Two would need an order between them that nothing
declares.

**One root element.** The marker naming the declaration is written onto it, and
a component with two roots has nowhere to put it.

**A module, and imports are absolute.** The block is extracted to a
content-hashed file under your public tree, so a relative specifier would
resolve against a directory the file no longer sits in. Import by URL; that is
also how two components share code.

## `setup` runs per instance; the module is evaluated once

This is the distinction the whole feature rests on, and it is worth being exact
about because it is the thing a mental model gets wrong.

The extracted file is an ES module. Its top level runs **once per URL, for the
life of the document** — a second `<script type="module">` for a URL already
evaluated does not re-run it, and neither does removing the tag and adding it
back. So module scope is the wrong place for anything belonging to one instance
or one visit:

```js
let count = 0;                       // shared by every instance, forever
export function setup({ el }) {
	let ownCount = 0;                  // this instance's
}
```

What runs per instance is the exported function, and everything it needs arrives
in the one object it is handed. Destructure what you use and ignore the rest:

```js
export function setup({ el, teardown, onSignal, props }) { }
```

Taking one object rather than a list of arguments is what lets a later capability
be one more key instead of a fourth parameter nobody passed.

## Release happens before the replacement lands

When a [partial update](/guides/cross-layer/partial-updates/) or a live delivery
replaces a region, the runtime tears down every component script inside it
first, then applies the new markup, then starts whatever arrived.

The ordering is deliberate. A teardown that ran afterwards would be reaching for
nodes that are already detached — `el.querySelector` returning `null`, a
`ResizeObserver` unobserving something that is gone.

It also means you can rely on the element still being there:

```js
export function setup({ el, teardown }) {
	const observer = new ResizeObserver(() => reflow(el));
	observer.observe(el);
	teardown(() => observer.disconnect());   // el is still in the document here
}
```

`teardown` registers rather than returns, so you can call it more than once, and
from inside a helper. Registrations run in reverse, last one first.

An operation that moves a region without replacing it — reordering a list —
releases nothing, because nothing was destroyed. The instances travel with their
nodes.

## `onSignal`, for what the server pushes

Anything registered through `onSignal` is released with the instance:

```js
export function setup({ el, onSignal }) {
	onSignal("app.finished", (event) => el.classList.add("done"));
}
```

It registers the handler in the [signal](/guides/cross-layer/signals/) table for
this instance, and the runtime releases every such registration before it runs
your teardowns. Keep component handlers on this surface: it prevents a destroyed
instance from leaving behind a callback that fires twice, then eventually twenty
times.

It is called `onSignal` and not `on` because `on-click` in a template binds a DOM
event, and a bare `on()` beside it would read as doing the same thing when it
does something else entirely.

## Handlers the markup can name

What `setup` returns is the set of handlers this component publishes, and a
template names one on the element that triggers it:

```html
export component Counter(label: string): html {
<script component>
  export function setup({ el }) {
    let count = 0;
    const output = el.querySelector("output");
    return {
      increment() {
        count += 1;
        output.textContent = count;
      },
    };
  }
</script>
  <div>
    <output>0</output>
    <button on-click="increment">{label}</button>
  </div>
}
```

Generation resolves `increment` against what the block returns, so a rename that
misses one of the two fails the build at the attribute rather than in a browser.
The handler closes over that instance's own state, which is the reason it comes
from the return value rather than from a module-level export: twenty rows in a
loop each get their own.

Write the name and nothing else. `on-click="increment()"` is not a call site —
an argument list would be an expression, and what varies per element is read from
the DOM instead:

```html
<button on-click="remove" data-id={row.ID}>delete</button>
```

```js
remove(event) {
	const id = event.currentTarget.dataset.id;
}
```

Read it at event time rather than at mount. The markup is the source of truth,
so an update that re-rendered the row leaves the next event reading the new
value.

`onclick` is untouched and still means inline JavaScript. It also still does not
run under this framework's default `script-src 'self'`, which is the other reason
handlers arrive this way.

## Calling a server action

A gesture is not the only reason to mutate. A script that decides for itself —
after a confirmation dialog, when a drag settles, on a timer — calls the route's
own [server actions](/guides/interactivity/server-actions/) by name:

```js
export function setup({ el, actions }) {
	return {
		async remove(event) {
			if (!confirm("Delete this?")) return;
			await actions.Delete({ id: event.currentTarget.dataset.id });
		},
	};
}
```

`actions` holds one function per exported handler in the page's own route
package, so the name you write is the Go function's. Nothing names a URL: the
address holds a digest of the declaring directory, which is not something a
script could compute.

The argument is sent as JSON, which `pw.Parse` reads into the same input struct
a form posts, so one handler serves both. Call with no argument and no body is
sent at all.

What comes back depends on what the handler wrote. Regions are applied exactly
as a gesture's are; anything else is returned to you, since you asked for it and
have somewhere to put it:

```js
const created = await actions.Draft();   // the handler's JSON body
```

Two things it does not do. There is no in-flight marking, because no element was
activated — you started the call and know what it is waiting for. And the direct
address carries no path parameters, so a handler that needs one reads it from
what you sent.

This is also the shape to reach for when a mutation has to be **gated**. Put the
handler on the element and leave `server-action` off it: a template carrying
both runs the handler and then issues the action regardless, because there is no
cancellation channel. Deciding in JavaScript is what makes the decision visible.

## Parameters the block asks for

Destructure a component parameter from `props` and generation emits it onto the
element for the runtime to hand back:

```js
export function setup({ props: { deadline } }) { }
```

Only what you name crosses, which makes destructuring the declaration of what
this component publishes to the browser — so read `{price}` there as putting the
price in the DOM, where anyone can read and edit it. Treat anything that comes
back as untrusted, and sign what must not change.

An absent optional omits its key rather than arriving as `null`, so `"deadline"
in props` is the test for one. Values keep their JSON types, which a `dataset`
read cannot: a number stays a number.

They are the values at mount, not a live binding. A value that changes belongs in
an attribute the handler reads when it fires.

## What it costs

A component that declares a block is extracted to its own file, referenced from
the merged head as an ordinary module. So it is cached like any static asset,
and the reference is loaded once however many instances render.

The per-instance cost is one attribute on the component's root element naming
its declaration, and one `setup` call. Twenty rows are twenty copies of a
constant in the static markup, which compresses to nothing, and twenty calls.

## When not to write one

If the browser already does it, do not. A disclosure widget is `<details>`, a
modal is `<dialog>`, a tooltip is `popover` — all of them work with no script at
all and keep working when yours throws. The
[ladder](/guides/interactivity/overview/) is still the rule, and this feature
sits on the tier above those rather than replacing them.

If the region's content is what changes, that is a
[partial update](/guides/cross-layer/partial-updates/) or a
[live boundary](/guides/cross-layer/live-rendering/), not a script that rewrites
the DOM the server just sent.

And a page must still be usable without the script. A component script is an
enhancement over markup the server rendered — nothing about it re-runs when
scripting is off, and a page whose correctness depends on `setup` having run is
one a reload can break.
