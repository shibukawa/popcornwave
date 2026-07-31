---
title: Build Tool Configuration
description: Every key of popcornwave.toml — what pw generates, what pw dev runs beside your application, and where the migrations and stylesheet live.
sidebar:
  order: 3
---

`popcornwave.toml` sits at the project root and belongs to the `pw` command. It
describes the *project*: which directories generation reads, which compiler the
sources were written for, what runs beside the application during development.

It holds no runtime setting at all. Ports, pools, cookies, and log levels live
in `config.{APP_ENV}.toml` and are listed in
[Application Configuration](/reference/configuration/). The split is enforced
rather than conventional — a `server` or `session` table here is an error, and
so is a database connection string. The two files are read by two different
programs at two different times.

[`pw init`](/pw/project/init/) writes this file, and
[`pw add`](/pw/project/add/) edits it when you install a capability. Editing it
by hand is expected; the rules below are what the loader checks.

## `[project]`

| Key | Default | Meaning |
| --- | --- | --- |
| `name` | *(required)* | the project name, also the `OTEL_SERVICE_NAME` `pw dev` injects |
| `main` | *(required)* | the package [`pw build`](/pw/project/build/) compiles, e.g. `"./cmd/myapp"` |
| `toolchain` | `"tinygo"` | the compiler the sources were scaffolded for: `tinygo` or `go` |
| `database` | `"sqlite"` | the SQL dialect `.pw.sql` sources generate for: `sqlite`, `postgres`, or `mysql` |

Both `toolchain` and `database` reject any other value, and both have a default
that is history rather than preference: a project written before the key existed
could only have been TinyGo, and could only have been SQLite.

`database` is a *generation* input. It decides the dialect the generated Go
reads your SQL as; the engine the application actually connects to still comes
from the scheme of `middleware.rdb.dsn`. Keeping the two in agreement is on you
— nothing here can check a DSN it is forbidden to hold.

## `[generate]`

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
pages = []
```

Each purpose lists the directories [`pw generate`](/pw/project/generate/) may
read *for that purpose*, and nothing else. A `.pw.sql` file in a directory that
`queries` does not list is invisible to generation — which is why generation
warns about a `.pw.html` or `.pw.sql` sitting outside the purpose that owns it,
rather than silently picking it up.

| Key | Reads | Required |
| --- | --- | --- |
| `generate.handlers` | handler sources, for route and binding analysis | yes |
| `generate.templates` | `.pw.html` templates, including the document shell | yes |
| `generate.queries` | `.pw.sql` sources | yes |
| `generate.config` | configuration registrations | yes |
| `generate.pages` | [page tree](/advanced/discovered-routing/) roots | no |

Every key but `pages` is required, and `[]` is how a project says a purpose
generates nothing. A missing key cannot say that, which is why the empty list is
not the same as leaving it out. `pages` is the exception because every project
scaffolded before page trees existed has neither the key nor a tree.

The rules on the entries themselves:

- relative to the project, naming a directory that exists
- no duplicates, and no entry nested inside another entry of the same purpose —
  the inner one's sources would be planned twice, and the second plan would
  delete what the first wrote
- exactly one `generate.templates` entry holds the
  [document shell](/guides/frontend/templates/); a second is an error
- a `generate.pages` entry is a whole tree, so it is neither listed under
  `templates` or `handlers` nor nested with one of their entries

Directory names are defaults, not identity. Every consumer reads the purpose
list rather than the name, so renaming `handlers/` to `web/` is moving the
directory and editing one line. The generated package name follows the
directory, so the sources still compile.

## `[dev.watch]`

```toml
[dev.watch]
includes = []
excludes = []
```

[`pw dev`](/pw/project/dev/) already walks the module for rebuild inputs, so
unlike generation this has a working default and both keys are optional.
`includes` adds relative files or glob patterns the walk misses. `excludes`
skips a directory subtree, which is worth doing for a large tree that only makes
the walk slower.

## `[dev.idp]`

```toml
[dev.idp]
enabled = false
config = "devidp.toml"
port = 0
```

The [development identity provider](/productivity/dev-identity-provider/) `pw
dev` runs beside the application. `enabled = true` requires the roster file to
exist. `port = 0` reserves an available loopback port, which is the useful
default because `pw dev` injects the resolved issuer into the application — a
fixed number matters only to a client registered somewhere outside this project.

## `[dev.otel]`

```toml
[dev.otel]
enabled = true
port = 0
max = 0
```

The [telemetry viewer](/productivity/dev-telemetry-viewer/), and the one block
here that is on by default. `port = 0` reserves a loopback port and `pw dev`
injects the resolved endpoint, exactly as for `dev.idp`. `max` bounds the
records retained per signal; `0` keeps the viewer's own default.

Both `dev.idp` and `dev.otel` affect `pw dev` and nothing else.

## `[migration]`

| Key | Default | Meaning |
| --- | --- | --- |
| `dir` | `"migrations"` | where [migration files](/productivity/migrations/) live, relative to the project |
| `auto` | `true` | apply pending migrations at the start of `pw dev` |

`auto` enables that one step in the developer loop. It never causes an
application to migrate its own database at startup — that stays an explicit call
in your code, because the process that serves requests is rarely the one that
should be changing the schema.

`dir` is a tooling path. It tells `pw` where the files are and has no runtime
meaning.

## `[assets.tailwind]`

```toml
[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

Present when the project was scaffolded with [Tailwind](/guides/frontend/styling/),
absent otherwise. `input` and `output` must be different files, both relative to
the project.

`minify` is the odd one: [`pw build`](/pw/project/build/) minifies whatever the
key says, and `pw dev` never does. What the value actually feeds is
[`pw doctor`](/pw/project/doctor/), which reports an unminified stylesheet as a
readiness finding for a deployed environment. Leave it `true`.

Tailwind plugins are not configured here. They are `@plugin` declarations in the
CSS entry, resolved by the Tailwind CLI — Popcorn Wave passes the entry through
unchanged and holds no plugin registry.

## Rules that apply to the whole file

- **Unknown keys are errors.** A typo does not silently do nothing.
- **Relative paths resolve from the file's directory**, and an absolute path is
  rejected. The project moves as a unit.
- **Command flags override the file.** `pw migrate --dir=other` reads that
  directory for one run without editing anything.
- **This file locates the project.** Every command that acts on one — all of
  them except [`pw init`](/pw/project/init/) and `pw version` — finds the root
  by walking up from the working directory until it sees this file, so `pw`
  works from any subdirectory.
- **Runtime values are forbidden.** `server`, `session`, `security`,
  `middleware`, and `observability` tables belong to the other file, and a
  database connection string belongs there too. `project.database` names an
  engine, never a DSN and never a credential.
