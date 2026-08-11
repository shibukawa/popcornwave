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
  export function setup(el) {
    const label = el.querySelector("[data-remaining]");
    const timer = setInterval(() => {
      label.textContent = remaining(el.dataset.deadline);
    }, 1000);
    return () => clearInterval(timer);
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
let count = 0;              // shared by every instance, forever
export function setup(el) {
	let ownCount = 0;         // this instance's
}
```

What runs per instance is the exported function. That is why the teardown is
what `setup` returns rather than a second export: it almost always needs
`setup`'s own locals, and two exports would have to talk through module scope,
which is exactly the scope that outlives the instance.

## Release happens before the replacement lands

When a [partial update](/guides/cross-layer/partial-updates/) or a live delivery
replaces a region, the runtime tears down every component script inside it
first, then applies the new markup, then starts whatever arrived.

The ordering is deliberate. A teardown that ran afterwards would be reaching for
nodes that are already detached — `el.querySelector` returning `null`, a
`ResizeObserver` unobserving something that is gone.

It also means you can rely on the element still being there:

```js
export function setup(el) {
	const observer = new ResizeObserver(() => reflow(el));
	observer.observe(el);
	return () => observer.disconnect();   // el is still in the document here
}
```

An operation that moves a region without replacing it — reordering a list —
releases nothing, because nothing was destroyed. The instances travel with their
nodes.

## The second argument, for signals

A `setup` receives a scope alongside its element. Anything registered through it
is released with the instance:

```js
export function setup(el, scope) {
	scope.on("app.finished", (event) => el.classList.add("done"));
}
```

`scope.on` registers the handler in the [signal](/guides/cross-layer/signals/)
table for this instance. The runtime releases every registration made through
the scope before it runs the teardown returned by `setup`. Keep component
handlers on this scoped surface: it prevents a destroyed instance from leaving
behind a callback that fires twice, then eventually twenty times.

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
