---
title: 1. Getting started
description: Create a project, run it, change a page, and watch the compiler catch a template mismatch.
sidebar:
  order: 1
---

By the end of this tutorial's four chapters, `memoapp` is a memo application
with a form, a database table, and a login that keeps each person's notes to
themselves. It begins as one page that says hello.

This chapter creates that page, runs it, and changes it. Fifteen minutes,
roughly, most of it spent waiting for the first build.

:::note[Before you start]
Go 1.26 or later and the `pw` command — see [Installation](/start/installation/).
Devbox is optional; the project works either way, and the commands below say
where the two paths differ. Everything else the project needs, `pw init` writes.
:::

## 1. Create the project

```sh
pw init memoapp
```

Run it in whatever directory you keep projects in. The command creates
`memoapp/` and refuses to write into a directory that already has files in it,
so there is no way to scatter a scaffold over existing work.

Nothing is asked, because the defaults answer every question:
SQLite for the database, a Valkey service in the development environment, no
Tailwind, and no login. Chapters 3 and 4 use the database and add the login;
[`pw add`](/pw/project/add/) installs a capability you declined, so none of
these answers is permanent. Run `pw init` with no project name to see the same
choices as a wizard.

Writing the files is not the last step. `pw init` then runs `go mod tidy` and
`pw generate`, so the project compiles by the time it reports success:

```
Created memoapp

Not included: auth, tailwind
  pw add <capability> enables one later

  cd memoapp
  devbox shell
  pw dev
```

## 2. Run it

```sh
cd memoapp
devbox shell
pw dev
```

Without Devbox, skip `devbox shell` and run `pw dev` directly — it needs Go on
`PATH`, and nothing else.

One command covers the whole loop: [`pw dev`](/pw/project/dev/) starts the
services declared in `devbox.json`, generates code, applies pending migrations,
then builds and runs the application, restarting it whenever a watched file
changes. `Ctrl-C` stops all of it together.

One of those services is Valkey. Nothing in this tutorial connects to it — it
is pinned in `devbox.json` so that a project which later needs a cache or a
rate limiter has the server already running. A project that never will can drop
the package from `devbox.json`.

The application reports its startup once, ending with the address it accepted:

```
   .-.   .-.
 .(   ) (   ).    Popcorn Wave v0.1.0
(   o     o   )   started at 2026-07-28 09:12:31 JST
(    \___/    )   env dev · config.dev.toml
 '-.__.___.__-'

configuration
├─ middleware
│  └─ rdb
│     ├─ dsn      [REDACTED]  ← file
│     └─ enabled  true        ← file
└─ server
   └─ port  8080  ← file

listening on http://localhost:8080
```

The real tree is longer: it lists every resolved configuration key, framework
and application alike, marks the ones that came from somewhere other than the
built-in defaults, and redacts secrets such as `rdb.dsn`. What it is for, and
what it becomes when nothing is attached to a terminal, is described under
[Seeing what took effect](/guides/architecture/configuration/#seeing-what-took-effect).

Open <http://localhost:8080/>. The page says **Hello, World**. The scaffolded
handler also reads a query parameter, so
<http://localhost:8080/?name=Popcorn> greets you by name.

## The files you will actually edit

`pw init` wrote a couple of dozen files. Three of them matter today:

| File | What it holds |
| --- | --- |
| `handlers/home.pw.html` | the page template — what the browser shows |
| `handlers/home_handler.go` | the route, and the input it reads from the request |
| `templates/document.pw.html` | the shell every page is rendered inside |

`popcornwave.toml` is worth knowing by name: it records the project's toolchain,
its database engine, and which directories each generation purpose reads.
`config.dev.toml` is the runtime configuration for `APP_ENV=dev`, which is where
the port and the database DSN above came from.

Two more files are generated and then ignored until chapter 3:
`queries/users.pw.sql` and `migrations/00001_init.sql` are a working example of
the typed SQL layer, and nothing in this chapter or the next one reads them.
[`pw init`](/pw/project/init/#what-it-writes) lists the rest of the tree.

One rule applies across all of it. Every `.pw.html` and `.pw.sql` file compiles
into a `_pw_gen.go` file **beside its source**, and those are build output: Git
ignores them, VS Code hides them, and `pw generate` recreates them. Edit the
source; never the generated Go.

### The page

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

That is not Go, and it is not a runtime template either. `.pw.html` is a small
typed language of its own, and `pw generate` compiles this file into a Go
function `Home` and a parameter struct `HomeParams`. Type errors and unsafe HTML
insertions are caught while it compiles rather than when a request arrives.
[Templates](/guides/frontend/templates/) covers the language; the parameter list
is what matters here.

The `class` attribute is a pair of Tailwind utilities. Without Tailwind
installed they style nothing, which is why the heading looks plain. `pw add
tailwind` installs the toolchain when you want them to work — see
[Styling](/guides/frontend/styling/).

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

An ordinary `net/http` handler, with two framework calls in it. `pw.Parse` fills
`homeInput` from the request — here from `?name=`, falling back to the declared
default. `pw.WriteHTML` renders the fragment `Home` returned.

`mux` comes from `handlers/index.go`, which is three lines long:

```go
package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
```

Each handler file registers its own route in `init`, so adding a route means
adding a file rather than editing a table that every feature has to touch.

Notice what the handler does *not* mention: `doctype`, `html`, `head`, or
`body`. Those live in `templates/document.pw.html`, and `pw.WriteHTML` renders
the page fragment inside that shell. A page template contributes leaf content
only.

## 3. Change the page

Leave `pw dev` running. Edit `handlers/home.pw.html`:

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
<p>Served by Popcorn Wave.</p>
}
```

Save it. `pw dev` regenerates `home_pw_gen.go`, rebuilds, and restarts the
application; reload the browser and the paragraph is there.

## 4. Break it on purpose

Adding markup is safe by construction. Changing the component's *interface* is
where the generated boundary starts doing work — so change it, and watch what
happens. Rename the parameter:

```html
package handlers

export component Home(visitor: string): html {
<h1 class="text-3xl font-bold">Hello, {visitor}</h1>
}
```

`pw dev` regenerates, tries to rebuild, and stops:

```
handlers/home_pw_gen.go
# memoapp/handlers
handlers/home_handler.go:21:37: unknown field Name in struct literal of type HomeParams
```

The renamed parameter renamed the field of `HomeParams`, and the handler is
still filling in `Name`. A template and its caller disagreed about a name, and
the disagreement surfaced as a compile error on the line that has to change —
not as a blank spot in a page some time later. (`pw dev` prints one further
error below this one: it also failed to read the configuration out of a binary
it could not build. One cause, two messages.)

Fix the handler:

```go
	pw.WriteHTML(w, r, Home(HomeParams{Visitor: input.Name}))
```

Save. The build succeeds, the application restarts, and the page is back.

Now undo both edits — rename the parameter back to `name` and restore
`HomeParams{Name: input.Name}` — because the next chapter starts from the
scaffolded handler.

## When it does not start

| What you see | What to do |
| --- | --- |
| `listen tcp :8080: bind: address already in use` | another process holds the port, quite possibly an earlier `pw dev`. Stop it, or set `server.port` in `config.dev.toml` |
| `devbox: command not found` | install [Devbox](https://www.jetify.com/devbox/), or skip `devbox shell` and run `pw dev` with Go on `PATH` |
| `go mod tidy` fails while downloading | the module cache is empty and the network refused. Retry, or set `GOPROXY` to a proxy you can reach; the scaffold is already written, so `pw init` does not need to run again |
| a `.pw.html` error with a line and column | generation rejected the template. The message names the position and the rule — see [Templates](/guides/frontend/templates/#errors) |

## What you have now

- A project that runs, reloads on save, and answers on `/healthz`, `/readyz`,
  `/openapi.json`, and `/docs` as well as `/`.
- A page template compiled into a typed Go function, and a handler that calls it.
- A build that fails when the two disagree.

Chapter 2 turns that page into something worth submitting: a form, a POST
handler, and a validation rule that has to report itself to a person.

- [2. Forms and validation](/tutorial/forms/) — the next chapter.
- [Architecture](/start/architecture/) — the model behind what you just ran.
- [pw command](/pw/overview/) — every subcommand in full.
