---
title: Styling
description: Component-scoped styles, and enabling Tailwind CSS — including after the fact.
sidebar:
  order: 6
---

Two styling approaches coexist. Component styles come with the template system
and need no setup; Tailwind CSS is opt-in.

## Component styles

A component contributes its own head content, and its class names are scoped
automatically:

```html
export component Card(label: string): html {
<head>
<style>
.box { color: red }
</style>
</head>
<div class="box"><span>{label}</span></div>
}
```

Classes declared in the block are renamed and the matching `class` attributes
rewritten. Classes **not** declared there pass through untouched, which is
exactly what lets Tailwind utilities sit next to scoped rules in the same
attribute. See [Templates](/guides/templates/).

## Tailwind CSS

Popcorn Wave drives the **standalone** Tailwind binary. There is no
`package.json`, no `node_modules`, and no Node lockfile.

The easy path is at creation time:

```sh
pw init myapp --tailwind
```

### Enabling it later

If you created the project without `--tailwind`, four things have to be in
place. None of them is generated, so add them by hand.

**1. Make the `tailwindcss` binary available.**

`pw dev` and `pw build` look for `tailwindcss` on `PATH` and fail with a clear
message if it is missing. Add it to `devbox.json`:

```json
{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/0.14.2/.schema/devbox.schema.json",
  "packages": ["go@latest", "valkey@latest", "tailwindcss_4@4.1.18"],
  "shell": {"init_hook": ["echo 'Popcorn Wave development environment'"]}
}
```

Then re-enter the shell so the new package is on `PATH`:

```sh
devbox shell
```

Pinning the version is deliberate — an unpinned CSS toolchain turns a
reproducible build into a moving target. If you manage tools some other way,
any `tailwindcss` on `PATH` works.

**2. Create the stylesheet entry point.**

```css
/* assets/app.css */
@import "tailwindcss";
@source "../handlers";
@source "../templates";
```

The `@source` lines are what let Tailwind see class names inside your `.pw.html`
files — without them the output is nearly empty. Add one line per directory
that contains templates; for the layout in
[Project structure](/guides/project-structure/) that means `@source "../webroot";`.

The `@import "tailwindcss"` line is checked before the build runs, so a
malformed entry point is reported rather than silently producing empty CSS.

**3. Turn it on in `popcornwave.toml`.**

```toml
[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

`input` and `output` are relative to the project root and must differ. When
`enabled` is true and the paths are omitted, these two values are the defaults.

**4. Link the output from the document shell.**

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>My App</title>
  <link rel="stylesheet" href="/public/generated/app.css">
</head>
<body><slot /></body></html>
}
```

The URL is `server.public.mount` (default `/public`) followed by the path of
`output` inside `public/`.

Then start the dev server:

```sh
pw dev
```

### How it runs

| Command | Behaviour |
| --- | --- |
| `pw dev` | one unminified build, then a watcher; the input is re-watched if the watcher exits |
| `pw build` | one minified build, regardless of `minify` in the file |

Output is written to a temporary file and renamed into place, so a half-written
stylesheet is never served. `public/generated/app.css` is build output — the
scaffolded `.gitignore` already excludes the compressed `public/**/*.zstd`
sidecars, and you will usually want to ignore the generated CSS too.

### Plugins

Local plugins referenced from the entry point are resolved relative to it and
verified to exist before the build:

```css
@import "tailwindcss";
@plugin "./plugins/typography.mjs";
@source "../handlers";
```

Keep the modules in the project — `assets/plugins/*.mjs` is the scaffolded
convention — so the build stays reproducible without a package manager.
