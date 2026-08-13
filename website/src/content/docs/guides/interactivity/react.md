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
element can do that, but Popcorn Wave already has the lifecycle needed here:
a [component script](/guides/interactivity/component-scripts/) runs `setup` for
each rendered instance and runs its teardown before that instance is replaced.

## Dependencies and the script build

React is an npm dependency at build time. Popcorn Wave's asset pipeline bundles
it into the browser entry before the application binary embeds the result.

```bash
npm install react react-dom
npm install --save-dev typescript @types/react @types/react-dom
```

Use a project-root `tsconfig.json` for both type-checking and the JSX transform.
Merge these keys into an existing configuration when the project already has one:

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

Point a module script at the authored React entry. The initial document loads
the bundle; the island's component script below owns each mount and teardown.

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

The build bundles and minifies the entry together with `react` and `react-dom`,
writes a source map, gives the result a content hash, and rewrites the script
URL. The JSX transform comes from `tsconfig.json`. Node.js and `node_modules`
remain build inputs; they are not deployed beside the application binary.

The transform removes TypeScript syntax but does not type-check it. Run
`tsc --noEmit` separately in CI.

## Put the lifecycle beside the server markup

Give the island useful fallback HTML. Until the script runs, this one shows the
current value and honestly leaves its control disabled:

```html
export component CounterIsland(initial: int): html {
<script component>
export function setup({ el, teardown }) {
  return window.mountCounter(el, Number(el.dataset.initial ?? "0"));
}
</script>
<section class="counter" data-initial={initial}>
  <button type="button" disabled>Count: {initial}</button>
</section>
}
```

The bundled entry provides that application-owned mount function. It returns
the cleanup that the component script hands back to the runtime:

```tsx
// public/islands/counter.tsx
import { useState } from "react";
import { createRoot } from "react-dom/client";

function Counter({ initial }: { initial: number }) {
  const [count, setCount] = useState(initial);
  return (
    <button type="button" onClick={() => setCount((value) => value + 1)}>
      Count: {count}
    </button>
  );
}

declare global {
  interface Window {
    mountCounter(el: HTMLElement, initial: number): () => void;
  }
}

window.mountCounter = (el: HTMLElement, initial: number) => {
  const root = createRoot(el);
  root.render(<Counter initial={initial} />);
  return () => root.unmount();
};
```

`mountCounter` is an application bridge, not a Popcorn Wave API; its only job is
to let the generated component module call the bundle whose URL the asset build
owns.

Popcorn Wave owns the `<section>` element and its placement. React owns the
element's children after `setup` mounts the root. The headings, forms, and lists
around it do not need to enter a React root.

The runtime calls `setup` on the initial page and on an instance inserted by a
partial or live update. Before an ancestor is replaced, it calls the returned
function, which unmounts the React tree and releases effects, subscriptions,
and handlers. The [component-script lifecycle](/guides/interactivity/component-scripts/#release-happens-before-the-replacement-lands)
therefore stays aligned with the server's DOM lifecycle.

Splitting the React component into another TypeScript file is useful when
several islands share it. Import that module from `counter.tsx`; keep `setup` in
the template declaration, because it owns the mount point and its teardown.

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
| placing `.counter` and writing `data-initial` | the Popcorn Wave template |
| children of `.counter` after `setup` | React |
| swapping lists or forms outside the island | htmx or application swap code |
| re-rendering a region containing the whole island | the server; the old island unmounts and the new one mounts |

Do not point `hx-target` at a button or another child React created. To refresh
initial server data, return a fragment containing the whole island. The
component-script teardown replaces the old root with the new one.

`pw.WriteHTMLFragment` may return the island markup, but a fragment cannot add
the React bundle to the head. The initial page must already have loaded
`counter.tsx`; later island instances reuse it and run their own `setup`.

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
