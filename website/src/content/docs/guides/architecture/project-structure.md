---
title: Project structure
description: Growing past a single handlers package — nested handler and query packages, and what popcornwave.toml controls.
sidebar:
  order: 1
---

`pw init` begins with one `handlers` package and one `queries` package. That
layout is easy to understand, but an application eventually develops separate
areas with separate owners. The question is how to split them without creating
a second framework-level registry.

## What generation reads

`pw generate` is scoped per purpose: each kind of generated code names the
directories it may come from, and reads nothing else.

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
```

`handlers` appears twice on purpose — a page template lives beside the handler
that renders it, so that directory serves both. Splitting the scope this way is
what lets the config purpose reach `cmd/myapp` without the handler purpose
scanning it too. See [`pw generate`](/pw/project/generate/) for what each
purpose reads and emits.

No purpose has a default: a missing key is an error, and `[]` is how a project
says the purpose generates nothing. What generation reads is a line you can
read rather than a walk you have to reason about.

Nesting inside a listed directory costs nothing — `webroot/admin/queries` is
already covered by a `webroot` entry. Adding a new **top-level** source
directory is the case that needs an edit here.

A source outside the purpose that owns it is reported and skipped rather than
failing the build, so deliberate samples and fixtures can live beside your code:

```
pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it
```

## A larger layout

Once an application serves distinct audiences, a common layout looks like this:

```
myapp/
├── popcornwave.toml
├── cmd/myapp/main.go
├── templates/
│   ├── document.pw.html          the one document shell
│   └── 400|404|500.pw.html
├── migrations/
├── webroot/
│   ├── index.go                  root mux; mounts the sub-applications
│   ├── home_handler.go
│   ├── home.pw.html
│   ├── admin/
│   │   ├── index.go              admin mux
│   │   ├── dashboard_handler.go
│   │   ├── dashboard.pw.html
│   │   └── queries/
│   │       └── reports.pw.sql
│   └── public/
│       ├── index.go              public mux
│       ├── signup_handler.go
│       ├── signup.pw.html
│       └── queries/
│           └── accounts.pw.sql
└── queries/
    └── users.pw.sql              queries shared by more than one area
```

Handlers and the templates they render stay in the same directory, and each
area owns the queries only it uses. Queries used by several areas move up to a
shared package. Ownership is visible from the path, which is the point.

### Each area owns a mux

A leaf package looks exactly like the scaffolded `handlers` package:

```go
// webroot/admin/index.go
package admin

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
```

```go
// webroot/admin/dashboard_handler.go
package admin

func init() { mux.HandleFunc("GET /dashboard", dashboard) }
```

Paths registered here are **relative to where the area is mounted**.

### The root mounts them

```go
// webroot/index.go
package webroot

import (
	"net/http"

	"myapp/webroot/admin"
	"myapp/webroot/public"

	"github.com/shibukawa/popcornwave/pw"
)

var mux = pw.NewServeMux()

func init() {
	mux.Handle("/admin/", http.StripPrefix("/admin", admin.Handlers()))
	mux.Handle("/", public.Handlers())
}

func Handlers() *pw.ServeMux { return mux }
```

```go
// cmd/myapp/main.go
func main() {
	if err := pw.Run(context.Background(), webroot.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

The dependency direction makes the composition work. The parent imports its
children, so Go runs each child's `init` function — and registers its routes —
before the parent's. Because a child never imports the parent, the packages do
not form a cycle.

Subtree patterns (`"/admin/"`) and `http.StripPrefix` are plain `net/http`;
nothing framework-specific is involved.

:::caution
Avoid mounting an area at `/public/`. The framework serves embedded static
assets at `server.public.mount`, which defaults to `/public`, and a collision
between your routes and an enabled operational endpoint is reported at startup.
Either mount the area elsewhere or move the asset mount in
[Configuration](/guides/architecture/configuration/).
:::

## What stays global

Three things do not shard, no matter how many packages you add:

**The document shell.** A project has exactly one `document.pw.html`; more than
one anywhere in the tree is a generation error. To give an area a different
shell, write an ordinary exported component with an unnamed slot and select it
per handler with `pw.WriteHTMLChain` — see [Templates](/guides/frontend/templates/).

**Migrations.** One ordered set for the whole application, in `migration.dir`.

**Configuration prefixes.** Each area can register its own configuration struct,
but the prefixes share one namespace — see
[Configuration](/guides/architecture/configuration/).

## `popcornwave.toml`

The project file is small, and its keys are a **closed set**: an unknown key is
an error rather than a warning.

```toml
[project]
name = "myapp"
main = "./cmd/myapp"
toolchain = "tinygo"
database = "sqlite"

[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]

[dev.watch]
includes = []
excludes = []

[migration]
dir = "migrations"
auto = true

[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

| Key | Default | Meaning |
| --- | --- | --- |
| `project.name` | — | required |
| `project.main` | — | required; the main package `pw build` and `pw dev` build |
| `project.toolchain` | `tinygo` | the compiler the project was scaffolded for; see [pw init](/pw/project/init/#changing-the-toolchain) |
| `project.database` | `sqlite` | the engine `.pw.sql` sources are generated for: `sqlite`, `postgres`, or `mysql`; see [Choosing the database](/pw/project/init/#choosing-the-database) |
| `generate.handlers` | — | required; directories read for routes and binding |
| `generate.templates` | — | required; directories read for `.pw.html` |
| `generate.queries` | — | required; directories read for `.pw.sql` |
| `generate.config` | — | required; directories read for config registration |
| `dev.watch.includes` | `[]` | extra relative glob patterns for `pw dev` |
| `dev.watch.excludes` | `[]` | subtrees `pw dev` skips while walking |
| `migration.dir` | `migrations` | migration directory, relative to the project |
| `migration.auto` | `true` | apply pending migrations when `pw dev` starts |
| `assets.tailwind.*` | disabled | see [Styling](/guides/frontend/styling/) |

The larger layout above needs one edit: `webroot` replaces `handlers` in the
`handlers` and `templates` purposes, since the areas nested inside it come along
for free.

Three other keys tend to move as an application grows. `dev.watch.includes`
picks up edited files the walk would otherwise miss. `dev.watch.excludes` trims
it when a large dependency tree makes the walk the slowest step of the loop.
`migration.auto` goes off when migrations should run under your own control.

`pw dev` deliberately watches wider than generation: any Go source is a rebuild
input, including files no purpose generates from. That is why its scope is
trimmed with excludes rather than declared with includes.
