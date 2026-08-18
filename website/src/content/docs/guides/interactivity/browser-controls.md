---
title: Browser controls
description: Dialogs, popovers, and disclosure widgets that need no JavaScript — and the four template rules that apply when they live in a .pw.html component.
sidebar:
  order: 2
---

:::note[Browser features]
`<dialog>`, the popover attributes and `<details>` belong to the web platform,
not to Popcorn Web. The framework's share of this page is the four template
rules at the end, which govern how that same markup behaves inside a `.pw.html`
component.
:::

Browsers now provide native elements and attributes for many controls that once
required a JavaScript library. Modals, menus, tooltips, and accordions can
therefore begin as markup in a server-rendered component.

The examples below use `<dialog>`, the popover attributes, and `<details>`.
Inside a `.pw.html` component they remain browser features, but four template
rules affect how you write them. Use a client-side library instead when the
native control cannot provide the interaction or accessibility behavior your
product needs.

## Dialogs

`<dialog>` gives you the top layer, a backdrop, `Esc` to close, and — with
`showModal()` — focus moved into the dialog and the rest of the document made
inert. None of that is yours to implement:

```html
export component DeleteButton(id: string, title: string): html {
<button type="button" command="show-modal" commandfor="confirm-delete">Delete</button>

<dialog id="confirm-delete" class="confirm">
  <form method="dialog">
    <p>Delete “{title}”?</p>
    <button value="cancel">Cancel</button>
  </form>
  <form method="post" action="/tasks/delete">
    <input type="hidden" name="id" value={id}>
    <button type="submit">Delete</button>
  </form>
</dialog>
}
```

Two forms, two outcomes. `method="dialog"` closes the dialog without submitting
anything and records the button's `value` as `returnValue`. The other is an
ordinary POST that leaves the page — after which the dialog is gone because the
document is new. A confirmation flow needs no state and no script.

The id travels in a hidden field rather than in the path because `action` is a
URL attribute: interpolating a `string` into it is a generation error, and the
type it wants is `url`. Building that value in the handler is the alternative;
a hidden field is usually less ceremony for a form that already exists.

`command="show-modal"` with `commandfor` opens it declaratively. That pair
landed across engines during 2025, so installed browsers still exist without
it; when the button must work everywhere, open it the old way instead:

```html
<button type="button" data-opens="confirm-delete">Delete</button>
```

```js
// public/dialogs.js
document.addEventListener('click', (event) => {
  const id = event.target.closest('[data-opens]')?.dataset.opens;
  if (id) document.getElementById(id)?.showModal();
});
```

One delegated listener covers every dialog on the site, including ones swapped
in later, because it resolves the target at click time rather than binding to
elements at load.

Use `showModal()` — or `command="show-modal"` — rather than `show()` unless you
specifically want a non-modal dialog. Only the modal form moves focus and makes
the background inert.

## Popovers

A popover is the same top-layer machinery without the modality, and it needs no
script at all:

```html
<button popovertarget="account-menu">Account</button>
<nav id="account-menu" popover class="menu">
  <a href="/profile">Profile</a>
  <a href="/settings">Settings</a>
  <form method="post" action="/logout"><button type="submit">Log out</button></form>
</nav>
```

The default `popover` (equivalent to `popover="auto"`) light-dismisses: clicking
outside or pressing `Esc` closes it, and opening another auto popover closes
this one. That is the behaviour a menu wants and the behaviour a dropdown
library spends most of its code on.

| Value | Closes when | Good for |
| --- | --- | --- |
| `popover` / `popover="auto"` | outside click, `Esc`, another auto popover opens | menus, dropdowns, disclosure panels |
| `popover="manual"` | only when told to | toasts, anything that must survive a click elsewhere |
| `popover="hint"` | another popover opens, or the trigger loses focus | tooltips, hover cards |

`popovertargetaction="show"`, `"hide"` or `"toggle"` picks what the trigger
does; `toggle` is the default.

A popover does **not** trap focus and does not make the rest of the page inert.
That is the correct trade for a menu and the wrong one for a destructive
confirmation. Reach for `<dialog>` when the answer matters.

### Positioning

Left alone, a popover renders in the centre of the viewport. CSS anchor
positioning ties it to its trigger without a positioning library:

```html
<head>
<style>
.trigger { anchor-name: --account }
.menu { position-anchor: --account; position-area: bottom span-right }
</style>
</head>
```

Anchor positioning is not in every engine yet, so treat it as an enhancement:
give the popover a workable position without it — a menu bar item can simply
place its panel with ordinary `position: absolute` inside a
`position: relative` wrapper — and let anchoring improve the placement where it
is supported.

## Disclosure and accordions

```html
<details name="faq" class="entry">
  <summary>How are migrations applied?</summary>
  <p>…</p>
</details>
<details name="faq" class="entry">
  <summary>Where does configuration come from?</summary>
  <p>…</p>
</details>
```

A shared `name` makes the group exclusive: opening one closes the others, which
is the whole of an accordion. Without `name` each `<details>` is independent,
which is usually what a sidebar wants.

The parameter is a `bool`, so an open-by-default section is data like anything
else:

```html
export component Section(label: string, expanded: bool, children: html): html {
<details class="entry" open={expanded}>
  <summary>{label}</summary>
  <slot />
</details>
}
```

Boolean attributes are emitted only when true, so `expanded` being false leaves
no `open` attribute at all.

## Template rules that apply here

Everything above is ordinary HTML, and none of it needed a rule explained first.
That changes as soon as the markup grows the two things it is missing: styles of
its own, and a list to repeat itself over. Two of the four rules below are
generation errors, one is a 500, and the last one is silent — which is also the
order in which they deserve your attention.

### A scoped selector needs a class

Component styles are scoped by renaming the classes you declare, so a selector
with no class in it has nothing to scope. This fails generation:

```
selector "dialog::backdrop" has no class to scope; add a class or wrap it in :global()
```

Both fixes are one line. Prefer the first — it keeps the rule scoped:

```html
<head>
<style>
.confirm::backdrop { background: rgb(0 0 0 / 0.4) }
.confirm[open] { animation: pop 120ms ease-out }
.menu:popover-open { display: grid }
:global(dialog::backdrop) { background: rgb(0 0 0 / 0.4) }
</style>
</head>
```

`::backdrop`, `[open]`, `:popover-open` and `:modal` all work; they just have to
hang off a class you declared. `@media` and `@supports` blocks scope their
contents the same way. See [Styling](/guides/frontend/styling/) for the general rule.

### Styles for a swapped region belong to the page

A component's `<head>` block reaches the document only on the page path.
`pw.WriteHTMLFragment` has no head to merge into and answers 500 rather than
dropping the contribution silently.

So a dialog that arrives by swap cannot bring its own styles. Declare them in a
component the page already rendered, or in the shared stylesheet. Details are in
[Fragments and islands](/guides/interactivity/fragments/).

### A brace in an inline script may be an insertion

Ordinary JavaScript inside a `<script>` element goes through untouched — class
bodies, function bodies, object literals, a JSON block for a `type` the browser
reads rather than executes. What still opens a template insertion is a brace
whose contents read as an expression, and JavaScript has one construct that
looks exactly like that:

```html
<script type="module">
const shorthand = {label};
</script>
```

The error names the position and the ways out of it:

```
unknown identifier label; this is inside <script> content, where {...} is a
template insertion. Write {{...}} to keep a literal brace, insert a value with
RawJavaScript or JsonForScript, or move the script to a file under the public
asset directory
```

Shorthand property syntax is the common collision; a `{name}` inside a string
literal is the other. Doubling the braces keeps them literal, and inserting a
real value is what the two script intrinsics are for:

```html
<script type="module">
const config = {JsonForScript(settings)};
</script>
```

Use `JsonForScript` for typed data and reserve `RawJavaScript` for fixed
strings — it is not a sanitiser, and [Templates](/guides/frontend/templates/)
has the escaping rules. Moving the script into `public/` avoids the question
altogether, which is what a `script-src` policy wants anyway:

```html
<script type="module" src="/public/copy-button.js"></script>
```

One position sits outside all of this: a component's `<head>` **contribution
block** is taken verbatim, so `{label}` there is neither an insertion nor an
error. It is the literal text `{label}`, delivered to the document head.

### One dialog, not one per row

`command`/`commandfor` and `popovertarget` resolve by `id`, and a `for` loop
that emits a dialog per row emits the same `id` per row. Every button then
opens the first one.

Give the id the row's identity:

```html
{for task in tasks}
  <li>
    <span>{task.title}</span>
    <button type="button" command="show-modal" commandfor="confirm-{task.id}">Delete</button>
    <dialog id="confirm-{task.id}" class="confirm">…</dialog>
  </li>
{/for}
```

Or — usually better for a long list — render one dialog outside the loop and
let the interaction that opens it fill in what it needs. That is the shape the
[fragment recipes](/guides/interactivity/fragments/) use.

## Accessibility notes

- `showModal()` handles focus and inertness. `show()` and popovers do not; if
  you need the rest of the page unavailable, use the modal form rather than
  reimplementing it with the `inert` attribute.
- A `<summary>` is already a button to assistive technology. Do not put another
  one inside it.
- A region that changes without a navigation should say so. Mark it
  `aria-live="polite"` — the swap recipes in
  [Fragments and islands](/guides/interactivity/fragments/) depend on it.
- Anything that animates should respect `prefers-reduced-motion`.

Only the last of those is work. The rest is a description of what `<dialog>`,
`<summary>` and the popover attributes already do — which is the strongest
argument for writing these elements rather than assembling the same widget out
of `div`s and listeners further up the ladder.
