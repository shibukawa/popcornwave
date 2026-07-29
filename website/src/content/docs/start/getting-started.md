---
title: Getting started
description: Create a project with pw init, run it with pw dev, and make your first change.
sidebar:
  order: 2
---

An empty directory is only a few commands away from a running application. This
walkthrough creates that application, examines what was generated, and then
changes a page to test the development loop.

## 1. Create the project

```sh
pw init myapp
```

The project name may contain letters, digits, `-`, and `_`. More importantly,
`pw init` refuses to write into a directory that already contains files. An
accidental `pw init .` in a populated tree therefore fails before it can scatter
new files.

Adding `--tailwind` scaffolds Tailwind CSS as well — an `assets/app.css` entry
point, the `[assets.tailwind]` block in `popcornwave.toml`, a pinned
`tailwindcss` package in Devbox, and a stylesheet link in the document shell.
No `package.json` and no Node lockfile are created.

```sh
pw init myapp --tailwind
```

Once the files are in place, `pw init` runs `go mod tidy` and `pw generate`.
The generated project therefore compiles before the command reports success:

```
Created myapp

  cd myapp
  devbox shell
  pw dev
```

## 2. What you get

```
myapp/
├── popcornwave.toml           project name, main package, generation sources
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

Every `.pw.html` and `.pw.sql` file becomes a `_pw_gen.go` file **next to its
source**. These generated files are build output: Git ignores them, VS Code
hides them, and `pw generate` recreates them. Edit the source, never the
generated Go.

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

The handler calls `Home` and `HomeParams`, yet neither appears in handwritten
Go. Both come from `handlers/home.pw.html`:

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

The handler never mentions the document shell. `pw.WriteHTML` takes the page
fragment and renders it inside the document registered from
`templates/document.pw.html`; [Templates](/guides/templates/) explains that
composition in detail.

## 3. Run it

```sh
cd myapp
devbox shell
pw dev
```

`pw dev` starts the Devbox services, runs `pw generate`, applies pending
migrations, starts the Tailwind watcher when it is enabled, and then builds and
runs the application — restarting it whenever a watched file changes.

The application reports its startup once, as a tree, and ends with the address
it accepted:

```
handlers/home_pw_gen.go
queries/users_pw_gen.go
version 0 -> 1

   .-.   .-.
 .(   ) (   ).    Popcorn Wave v0.1.0
(   o     o   )   started at 2026-07-28 09:12:31 JST
(    \___/    )   env dev · config.dev.toml
 '-.__.___.__-'

configuration
├─ middleware
│  ├─ rdb
│  │  ├─ dsn             [REDACTED]  ← file
│  │  ├─ enabled         true        ← file
│  │  └─ max_open_conns  1           ← file
│  └─ request_timeout  0s
├─ observability
│  ├─ minimum_level  debug  ← file
│  └─ service_name   myapp  ← file
└─ server
   ├─ api_doc  scalar  ← file
   └─ port     8080    ← file

listening on http://localhost:8080
```

Abbreviated here: the real tree lists every resolved key, framework and
application alike. Only values that came from somewhere other than the built-in
defaults are marked — `← file` above, and `← env` or `← flag` elsewhere — and
keys such as `rdb.dsn` are redacted. Away from a terminal the same facts become
a single structured log record instead; see
[Startup summary](/guides/configuration/#startup-summary).

Open <http://localhost:8080/>. The scaffolded page also responds to a query
parameter, so <http://localhost:8080/?name=Popcorn> greets you by name.

The scaffolded `config.dev.toml` names the operational endpoints, so a new project also answers on:

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

Save the file. `pw dev` regenerates `home_pw_gen.go`, rebuilds the application,
and restarts it. Reload the browser to see the new paragraph.

Changing the component's *signature* has a different effect. If it becomes
`Home(name: string, count: int)`, the generated `HomeParams` changes too, and
the handler stops compiling until it supplies `Count`. The reload loop has
exposed a contract mismatch at build time rather than at runtime.

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
