---
title: pw dev
description: The development loop — services, generation, migrations, CSS, and restart on change.
sidebar:
  order: 3
---

```sh
pw dev
```

The everyday command. It takes no arguments.

## What it does on startup

1. starts the Devbox services declared in `devbox.json`;
2. runs [`pw generate`](/pw/project/generate/);
3. applies pending migrations, unless `migration.auto` is `false`;
4. builds the Tailwind stylesheet and starts its watcher, if Tailwind is enabled;
5. builds and runs `project.main`.

Then it polls the watched files twice a second and repeats the relevant steps
when something changes.

## What it watches

- the project's own Go, `.pw.html`, and `.pw.sql` sources;
- the migration directory;
- the Tailwind input, when Tailwind is enabled;
- anything matched by `dev.extra_watch` in `popcornwave.toml`.

`dev.extra_watch` takes relative glob patterns. Absolute paths are rejected:

```toml
[dev]
extra_watch = ["config.dev.toml", "assets/**/*.svg"]
```

## Tailwind

The watcher runs **unminified** during development regardless of
`assets.tailwind.minify`, because minification is the slow part of the loop. If
the watcher exits, `pw dev` keeps going and falls back to watching the input
file directly, so a crashed CSS process does not take the server down with it.

`tailwindcss` must be on `PATH`, which is what `devbox shell` is for. See
[Styling](/guides/styling/).

## Migrations

Pending migrations are applied before the application starts, and again when a
file in the migration directory changes. Turn this off when you would rather
control it yourself:

```toml
[migration]
auto = false
```

## Stopping

`Ctrl-C` cancels the run: the application, the Tailwind watcher, and the Devbox
services are stopped. If the application exits on its own with an error,
`pw dev` reports `application exited: …` and stops, rather than looping on a
process that cannot start.
