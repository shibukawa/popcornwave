---
title: Project structure
description: Growing past a single handlers package — nested handler and query packages, and what popcornwave.toml controls.
sidebar:
  order: 5
---

`pw init` begins with one `handlers` package and one `queries` package. That
layout is easy to understand, but an application eventually develops separate
areas with separate owners. The question is how to split them without creating
a second framework-level registry.

## What generation discovers

`pw generate` walks the whole project tree and generates into **every directory
that contains** a `.go`, `.pw.html`, or `.pw.sql` file — skipping `.git`,
`vendor`, `node_modules`, and `.devbox`.

That discovery rule does most of the work. Adding a package means creating a
directory; there is no package registry to update, and `popcornwave.toml` does
not enumerate the tree.

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
children, so Go runs each child's `init` function—and registers its routes—
before the parent's. Because a child never imports the parent, the packages do
not form a cycle.

Subtree patterns (`"/admin/"`) and `http.StripPrefix` are plain `net/http`;
nothing framework-specific is involved.

:::caution
Avoid mounting an area at `/public/`. The framework serves embedded static
assets at `server.public.mount`, which defaults to `/public`, and a collision
between your routes and an enabled operational endpoint is reported at startup.
Either mount the area elsewhere or move the asset mount in
[Configuration](/guides/configuration/).
:::

## What stays global

Three things do not shard, no matter how many packages you add:

**The document shell.** A project has exactly one `document.pw.html`; more than
one anywhere in the tree is a generation error. To give an area a different
shell, write an ordinary exported component with an unnamed slot and select it
per handler with `pw.WriteHTMLChain` — see [Templates](/guides/templates/).

**Migrations.** One ordered set for the whole application, in `migration.dir`.

**Configuration prefixes.** Each area can register its own configuration struct,
but the prefixes share one namespace — see
[Configuration](/guides/configuration/).

## `popcornwave.toml`

The project file is small, and its keys are a **closed set**: an unknown key is
an error rather than a warning.

```toml
[project]
name = "myapp"
main = "./cmd/myapp"

[dev]
extra_watch = []

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
| `dev.extra_watch` | `[]` | extra relative glob patterns for `pw dev` |
| `migration.dir` | `migrations` | migration directory, relative to the project |
| `migration.auto` | `true` | apply pending migrations when `pw dev` starts |
| `assets.tailwind.*` | disabled | see [Styling](/guides/styling/) |

The larger layout requires **no change to this file**. As the application grows,
the keys most likely to change are `dev.extra_watch`, for generated or edited
files that `pw dev` would otherwise miss, and `migration.auto`, when migrations
should run under your own control.
