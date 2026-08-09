---
title: Integrating React
description: Keep the server-rendered page and give React ownership of only the small region that needs durable client-side state.
sidebar:
  order: 7
---

React can own one part of a Popcorn Wave page. The boundary is literal:
Popcorn Wave renders the document and surrounding HTML, while React manages
only the children of one element.

Mounting is the easy half. If a server fragment later replaces that element,
something must clean up the old React root and start the new one. A custom
element puts both operations on the browser's own connection lifecycle.

## Dependencies and the script build

React is an npm dependency at build time. Popcorn Wave's asset pipeline bundles
it into the browser entry before the application binary embeds the result.

```bash
npm install react react-dom
npm install --save-dev typescript @types/react @types/react-dom
```

Use a project-root `tsconfig.json` for both type-checking and the JSX transform.
Merge these keys into an existing configuration when the project already has
one:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "lib": ["DOM", "ES2020"],
    "strict": true,
    "noEmit": true
  },
  "include": ["public/**/*.ts", "public/**/*.tsx"]
}
```

Commit `package-lock.json`, then enable script conversion:

```toml
# popcornwave.toml
[assets.scripts]
enabled = true
```

Point a module script at the authored file. The tag is the whole registration:
there is no separate entry list to keep in step with it.

```html
export component TasksPage(initialCount: int): html {
<head>
  <script type="module" src="/public/islands/counter.tsx"></script>
</head>
<main>
  <h1>Tasks</h1>
  <CounterIsland initial={initialCount} />
</main>
}
```

When `pw build` sees this reference, it bundles and minifies the entry together
with `react` and `react-dom`, writes a source map, gives the bundle a content
hash, and rewrites the generated script URL to that immutable file. The JSX
transform comes from the `jsx` setting in `tsconfig.json`, which the build reads
itself. Node.js and `node_modules` remain build inputs; they are not deployed
beside the application binary.

The transform removes TypeScript syntax but does not type-check it. Run
`tsc --noEmit` separately in CI.

## Put a mount point in the server markup

Give the island useful fallback HTML. Until the script runs, this one shows the
current value and honestly leaves its control disabled:

```html
export component CounterIsland(initial: int): html {
<react-counter data-initial={initial}>
  <button type="button" disabled>Count: {initial}</button>
</react-counter>
}
```

Popcorn Wave owns the `<react-counter>` element and its placement. React owns
the element's children after mounting. The headings, forms, and lists around it
do not need to enter a React root.

## Keep the component and its lifecycle together

The component and the custom element that owns it belong together, because the
custom element is the only thing that decides when the component exists:

```tsx
// public/islands/counter.tsx
import { useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';

type CounterProps = { initial: number };

function Counter({ initial }: CounterProps) {
  const [count, setCount] = useState(initial);
  return (
    <button type="button" onClick={() => setCount((value) => value + 1)}>
      Count: {count}
    </button>
  );
}

class ReactCounterElement extends HTMLElement {
  root: Root | null = null;

  connectedCallback() {
    if (this.root) return;
    this.root = createRoot(this);
    this.root.render(<Counter initial={Number(this.dataset.initial ?? '0')} />);
  }

  disconnectedCallback() {
    this.root?.unmount();
    this.root = null;
  }
}

if (!customElements.get('react-counter')) {
  customElements.define('react-counter', ReactCounterElement);
}
```

Splitting this across files is worth it when several islands share a component,
not before. The bundler follows imports from the entry, so a shared
`components/counter.tsx` reaches the same bundle without any change to the tag.

An element in the initial page mounts through `connectedCallback`. The same
element inserted later by htmx or another swap library follows that path too.
When an ancestor is replaced, `disconnectedCallback` unmounts the React tree
and releases its effects, subscriptions, and handlers. There is no separate
"after swap" initializer to keep in sync.

This deliberately uses light DOM. The button React creates inherits the page's
stylesheet, Tailwind utilities, and theme. A shadow root would require the
island to reconnect all of that styling itself.

## Use `createRoot`, not `hydrateRoot`

The fallback button came from a Popcorn Wave template, not from React server
rendering. `createRoot` replacing its children on the first render is therefore
the intended operation.

Do not switch to `hydrateRoot` just because the two versions look alike.
Hydration expects markup generated from the same React tree by
`react-dom/server`, and mismatches are bugs. Defining one DOM shape in both a Go
template and a React component eventually produces warnings, lost input, or
misbound events.

If the server only supplies initial values, `data-*` attributes or a JSON
payload keep the boundary small. Real React SSR and hydration require a Node.js
renderer, React's streaming protocol, and a deployment boundary between it and
Go. Popcorn Wave does not provide that system.

## DOM ownership during fragment swaps

Two renderers updating the same nodes make both states unreliable. Keep this
division:

| Operation | Owner |
| --- | --- |
| placing `<react-counter>` and writing `data-initial` | the Popcorn Wave template |
| children of `<react-counter>` | React |
| swapping lists or forms outside the island | htmx or application swap code |
| re-rendering a region containing the whole island | the server; the old island unmounts and the new one mounts |

Do not point `hx-target` at a button or another child React created. To refresh
initial server data, return a fragment containing the whole island. Its custom
element lifecycle replaces the old root with the new one.

`pw.WriteHTMLFragment` may return the island markup, but a fragment cannot
contribute to the head. The initial page must already have loaded `counter.tsx`.
That is why the script contribution lives on `TasksPage`, not on
`CounterIsland` itself.

## Writing back to the server

A counter that exists only in the browser needs no fetch. When a React action
does write to the server, call an ordinary handler and either update React state
after success or replace the whole island with the returned fragment.

With `security.csrf.enabled = true`, unsafe requests must read the current
default `pw_csrf` cookie and send it as `X-CSRF-Token`; use the configured names
if the application changed them. Do this at request time rather than freezing
the value into props; another tab may rotate the session while the page remains
open. The cookie helper in
[Integrating htmx](/guides/interactivity/htmx/#unsafe-requests-and-csrf) can be
used directly in a `fetch` headers object.

[Static assets](/guides/frontend/static-assets/) covers the rest of what the
script build does to the file: hashing, source maps, and the conversions that
apply to everything else under `public`.
