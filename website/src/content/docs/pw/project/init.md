---
title: pw init
description: Create a runnable Popcorn Wave project.
sidebar:
  order: 1
---

```sh
pw init <project-name> [--tailwind]
```

Creates a complete, runnable project in a new directory named after the
project.

## Options

| Option | Effect |
| --- | --- |
| `--tailwind` | also scaffold the Tailwind CSS toolchain |

## Validation

The project name accepts letters, digits, `-`, and `_`; `.` and `..` are
rejected. The destination must be empty or absent — an accidental `pw init .`
in a populated tree fails rather than scattering files.

## What it writes

```
myapp/
├── popcornwave.toml           project name, main package, dev watch list
├── config.dev.toml            runtime configuration for APP_ENV=dev
├── go.mod
├── devbox.json / devbox.lock  Go + Valkey (+ tailwindcss with --tailwind)
├── cmd/myapp/main.go          calls pw.Run
├── handlers/
│   ├── index.go               the package-level mux and Handlers()
│   ├── home_handler.go        route registration and the net/http handler
│   └── home.pw.html           typed page template
├── templates/
│   ├── document.pw.html       shared document shell
│   ├── templates.go           package marker, present before first generation
│   └── 400|404|500.pw.html    error pages
├── queries/users.pw.sql       named SQL with a typed result
├── migrations/00001_init.sql  initial schema, in goose format
├── public/.keep               empty-tree sentinel; never served
├── public.go                  embeds public/ and registers it
├── .vscode/settings.json      hides **/*_pw_gen.go
└── .gitignore                 excludes *_pw_gen.go and other build output
```

With `--tailwind` it also writes `assets/app.css` and
`public/generated/app.css`, adds the `[assets.tailwind]` block to
`popcornwave.toml`, pins `tailwindcss` in `devbox.json`, and links the
stylesheet from the document shell. No `package.json` and no Node lockfile are
created. See [Styling](/guides/styling/) for enabling this later.

Files are written atomically — each to a temporary file, then renamed — so an
interrupted run cannot leave a half-written source file.

## What it runs

After writing the files, `pw init` runs `go mod tidy` and then
[`pw generate`](/pw/project/generate/), so the project compiles immediately. It
finishes by printing:

```
Created myapp

  cd myapp
  devbox shell
  pw dev
```

The generated `go.mod` requires the framework at the version of the `pw` binary
that created it. When `pw` was built from a working copy rather than a release,
it writes a `replace` directive pointing at that checkout instead.

## Next steps

- [Getting started](/start/getting-started/) — a walkthrough of the output.
- [Project structure](/guides/project-structure/) — growing past one package.
