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
Either mount the area elsewhere or move the asset mount — see
[Static Assets](/guides/frontend/static-assets/).
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

[dev.logs]
enabled = true
directory = ".log"

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
| `dev.logs.enabled` | `true` | capture `pw dev` application logs as local JSONL |
| `dev.logs.directory` | `.log` | capture directory, relative to the project root |
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

`dev.logs` belongs here rather than in `config.dev.toml`: it controls the local
developer process, not the deployed application. See
[Telemetry](/guides/architecture/telemetry/) for the file lifecycle and schema.

## The architecture Popcorn Wave is designed for

More packages do not automatically create better separation. In a Go
application built on `net/http` and `database/sql`, those packages already
provide strong, shared boundaries understood by the compiler, tools, libraries,
and other Go developers. Wrapping each of them in controllers, use cases,
repositories, and local interfaces can reproduce boundaries that already exist
without changing what the program does.

Clean Architecture remains useful when a real dependency must be reversed or
when separately owned code needs protection from change. Popcorn Wave rejects
the mechanical version: every application gets every ring, whether or not the
rings hold different knowledge. A layer must earn its place.

### Put domain knowledge where the data lives

Some design guidance inherited from enterprise Java treats the database as
contaminating infrastructure and the domain as pure in-memory logic. That split
is more than twenty years old, yet the older diagram is still reproduced as a
rule long after its original constraints have changed.

The database did not become less important. Its schema, keys, constraints,
relationships, indexes, queries, and transaction boundaries encode what the
application permits and what it can do efficiently. A transaction is not a
storage detail when moving its boundary changes atomicity, concurrency, and
failure behavior. Those are domain consequences.

Hiding those properties behind a generic CRUD repository tends to make the
important choices invisible. Code then fetches rows one at a time, joins them in
memory, loads columns it does not need, or opens transactions at a boundary
chosen for architectural symmetry rather than for the work. The result may look
pure while doing more database work and providing weaker guarantees.

Popcorn Wave therefore expects domain knowledge to reach the schema and SQL.
Go code orchestrates the request, external systems, and the rules SQL cannot
express clearly; it does not pretend the data model is outside the domain. This
is why generated queries keep their SQL visible and why transaction ownership
stays explicit in application code.

### Organize packages by feature, not by layer

A top level divided into `handlers`, `controllers`, `services`, `repositories`,
and `models` scatters one change across generic packages. It also creates the
same names repeatedly in editor tabs and completion results. Each boundary then
invites another request type, domain type, persistence type, and mapper between
them. The conversion code carries little new knowledge, but it lowers the
semantic density of the source, adds code and sometimes binary weight, and
consumes review time, human attention, and AI context alike.

Modern Java applications are not uniformly organized by layer either. Even so,
the layout keeps being reintroduced when diagrams from twenty-year-old books
are treated as a required project template rather than as a response to the
constraints of their time.

Russ Cox made the narrower underlying point when objecting to a repository
presented as a standard Go layout: the proposed structure was unusually
complex, while Go repositories tend to be much simpler. See
[“this is not a standard Go project layout”](https://github.com/golang-standards/project-layout/issues/117).
Popcorn Wave takes that preference for simplicity seriously.

The minimum layout for a small application is therefore shallow rather than
layered. The `pw init` scaffold has one handler area and one query package
because the application has only one feature area; it does not pre-create
controller, service, and repository tiers. Keep the handler, its template, and
its query code within a short path so one feature can be understood without
touring the repository. When the application grows, split by features such as
`admin`, `accounts`, or `billing`, as in the `webroot` tree above. Each feature
remains internally shallow; a parent mux composes the feature packages. Move
code upward only after more than one feature actually shares it.

Not every layer is artificial. Configuration input, an external HTTP request,
SQL, and HTML are genuine representation boundaries. Popcorn Wave supplies code
generation there because translating across them adds type and protocol checks.
It does not generate controller-to-service-to-repository glue or structure
mappers whose only job is to preserve an otherwise empty ring.

The goal is not the fewest possible packages. It is the fewest layers that carry
no distinct knowledge. Keep Go's standard interfaces visible, keep database
behavior explicit, and keep each feature coherent. Add a package when ownership
or a feature boundary becomes real—not in anticipation of a diagram.
