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

The built-in pipeline currently recognises `.ts`, but not `.tsx`, as a script
entry. Point a module script at the authored `.ts` file:

```html
export component TasksPage(initialCount: int): html {
<head>
  <script type="module" src="/public/islands/counter.ts"></script>
</head>
<main>
  <h1>Tasks</h1>
  <CounterIsland initial={initialCount} />
</main>
}
```

When `pw build` sees this reference, it bundles and minifies the entry together
with `react` and `react-dom`, writes a source map, gives the bundle a content
hash, and rewrites the generated script URL to that immutable file. Node.js and
`node_modules` remain build inputs; they are not deployed beside the
application binary.

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

## Separate the React component from its entry

The directly referenced entry has to be `.ts`, but esbuild follows imports from
that entry and already includes `.tsx` modules in the same bundle. The React
component can therefore use ordinary JSX:

```tsx
// public/islands/counter-view.tsx
import { useState } from 'react';
import { createRoot } from 'react-dom/client';

type CounterProps = { initial: number };

function Counter({ initial }: CounterProps) {
  const [count, setCount] = useState(initial);
  return (
    <button type="button" onClick={() => setCount((value) => value + 1)}>
      Count: {count}
    </button>
  );
}

export function mountCounter(element: Element, initial: number) {
  const root = createRoot(element);
  root.render(<Counter initial={initial} />);
  return root;
}
```

The thin `.ts` entry referenced by `.pw.html` owns only the custom-element
lifecycle:

```ts
// public/islands/counter.ts
import { mountCounter } from './counter-view';

class ReactCounterElement extends HTMLElement {
  root: ReturnType<typeof mountCounter> | null = null;

  connectedCallback() {
    if (this.root) return;
    const initial = Number(this.dataset.initial ?? '0');
    this.root = mountCounter(this, initial);
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
contribute to the head. The initial page must already have loaded `counter.ts`.
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

## Build support worth adding

The smallest high-value addition is to recognise **`.tsx` as a built script
entry**. Imported `.tsx` files already work, so React itself is not blocked.
The pipeline also already resolves npm imports, bundles, minifies, writes source
maps, and hashes output. Extending the entry gate to `.tsx`, with the same
module-tag check and reference rewriting, would remove the thin `.ts` shim.

After that, an optional `pw add react` could scaffold:

- `[assets.scripts]` and a `.tsx` entry;
- `react`, `react-dom`, type packages, and a `tsc --noEmit` script;
- a small custom-element wrapper that owns mount and unmount.

`pw build` should not silently install Node.js packages. The package manager
and lockfile are application supply-chain decisions. React SSR is also not a
prerequisite for partial React; it is a separate, much larger feature.

The useful order is therefore `.tsx` entry support first, an explicit scaffold
second, and SSR only in response to a real hydration use case. JSX-based partial
mounting already works; direct `.tsx` support would remove an entry file that
exists only to satisfy the current gate.
