---
title: Getting started
description: Create a project with pw init, run it with pw dev, and make your first change.
sidebar:
  order: 2
---

This walkthrough goes from an empty directory to a running application, then
changes a page and watches it reload.

## 1. Create the project

```sh
pw init myapp
```

The project name accepts letters, digits, `-`, and `_`. `pw init` refuses to
write into a directory that already has files in it, so an accidental
`pw init .` in a populated tree fails instead of scattering files.

Adding `--tailwind` scaffolds Tailwind CSS as well — an `assets/app.css` entry
point, the `[assets.tailwind]` block in `popcornwave.toml`, a pinned
`tailwindcss` package in Devbox, and a stylesheet link in the document shell.
No `package.json` and no Node lockfile are created.

```sh
pw init myapp --tailwind
```

After writing the files, `pw init` runs `go mod tidy` and then `pw generate`, so
the project compiles immediately. It finishes by printing:

```
Created myapp

  cd myapp
  devbox shell
  pw dev
```

## 2. What you get

```
myapp/
├── popcornwave.toml           project name, main package, dev watch list
├── config.dev.toml            runtime configuration for APP_ENV=dev
├── go.mod
├── devbox.json                Go + Valkey (+ tailwindcss with --tailwind)
├── cmd/myapp/main.go          calls pw.Run
├── handlers/
│   ├── index.go               the package-level mux and Handlers()
│   ├── home_handler.go        route registration and the net/http handler
│   └── home.pw.html           typed page template
├── templates/
│   ├── document.pw.html       shared document shell (doctype, html, head, body)
│   ├── templates.go           package marker, present before first generation
│   └── 400|404|500.pw.html    error pages
├── queries/users.pw.sql       named SQL with a typed result
├── migrations/00001_init.sql  initial schema, in goose format
├── public/.keep               empty-tree sentinel; never served
├── public.go                  embeds public/ and registers it
├── .vscode/settings.json      hides **/*_pw_gen.go
└── .gitignore                 excludes *_pw_gen.go and other build output
```

Every `.pw.html` and `.pw.sql` file is compiled into a `_pw_gen.go` file **next
to its source**. Those files are build output: they are git-ignored, hidden in
VS Code, and recreated by `pw generate`. You never edit them.

### The entry point

```go
package main

import (
	"context"
	"log"

	"myapp/handlers"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`pw.Run` owns configuration parsing, startup validation, the middleware stack,
serving, graceful shutdown, and reverse-order resource cleanup.

### The handler

```go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

type homeInput struct {
	Name string `query:"name" default:"World"`
}

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[homeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
}
```

`Home` and `HomeParams` do not exist in any file you can see — they are
generated from `handlers/home.pw.html`:

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

Note what the handler does *not* do: it never mentions the document shell.
`pw.WriteHTML` renders the page fragment inside the document registered from
`templates/document.pw.html`. See
[Templates](/guides/templates/).

## 3. Run it

```sh
cd myapp
devbox shell
pw dev
```

`pw dev` starts the Devbox services, runs `pw generate`, applies pending
migrations, starts the Tailwind watcher when it is enabled, and then builds and
runs the application — restarting it whenever a watched file changes.

Open <http://localhost:8080/>. The scaffolded page also responds to a query
parameter, so <http://localhost:8080/?name=Popcorn> greets you by name.

The framework also mounts, by default:

| Path | Purpose |
| --- | --- |
| `/healthz` | liveness |
| `/readyz` | readiness |
| `/openapi.json` | generated OpenAPI document |
| `/docs` | Scalar API documentation (development config only) |
| `/public/` | embedded static assets |

## 4. Make a change

Edit `handlers/home.pw.html`:

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
<p>Served by Popcorn Wave.</p>
}
```

Save it. `pw dev` regenerates `home_pw_gen.go`, rebuilds, and restarts. Reload
the browser.

If you instead change the component's *signature* — say `Home(name: string,
count: int)` — the generated `HomeParams` struct changes with it and the handler
stops compiling until you pass the new field. That is the point: template and
handler are checked against each other by the Go compiler.

## 5. Build for production

```sh
pw build
```

This regenerates code, builds minified CSS when Tailwind is enabled, prepares
the compressed sidecars for public assets, and runs `go build` on the main
package from `popcornwave.toml`.

Select the runtime environment with `APP_ENV`:

```sh
APP_ENV=prod ./myapp
```

## Next steps

- [Architecture](/start/architecture/) — the model the framework is built around.
- [Handlers](/guides/handlers/) and [Responses](/guides/responses/) — the `pw` API in full.
- [pw command](/pw/overview/) — every `pw` subcommand.
