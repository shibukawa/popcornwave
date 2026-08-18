---
title: Component Packages
description: Publishing a reusable capability as a Go module, and installing one by naming it in popcornweb.toml.
sidebar:
  order: 3
---

Copying an admin console, a middleware with its own tables, or a styled
component set into a second project creates two implementations to maintain.
Changes made in one copy soon stop reaching the other.

A **component package** is that capability published as an ordinary Go module.
The consuming project names it in one place:

```toml
[[packages]]
module = "example.com/widget"
```

That declaration is enough to install it. `pw generate` writes the import that
links the package, and `pw migrate up` creates its tables. No package source is
copied into the application.

:::note[Before you start]
This page assumes a project created by [`pw init`](/pw/project/init/). Publishing
a package needs no other capability; installing one may, if the package declares
that it does.
:::

## When not to reach for this

If what you are sharing is plain Go — no `.pw.html`, no migrations, no browser
assets — publish an ordinary Go module and stop reading. Everything below exists
to move the things Go modules alone cannot carry, and a package section on a
module that has none of them is a manifest nobody needs.

The other boundary is firmer: **a component in one module is not callable from
another module's template.** `.pw.html` can only reach components declared in
its own generation unit, so a package that exists to export components is not
something you can build today. Packages that ship handlers, a `pages/` tree, a
middleware, a schema, or browser assets work now.

## The package side

A package is a project with `kind = "package"` and no entry point, because the
application that imports it owns `main`:

```toml
[project]
name = "widget"
kind = "package"

[package]
module = "example.com/widget"
summary = "A note widget with its own storage"
assets.declared = true
routes.register = "Register"

[package.migrations]
dir = "migrations"
stem = "widget"
engines = ["sqlite"]

[package.generated_with]
pw = "v0.4.0"
tinybind = "v0.3.5"

[generate]
handlers = ["."]
templates = ["."]
queries = []
config = []
```

The Go side registers the module's identity and its embedded files from `init`,
so only a binary that actually links the package pays for any of it:

```go
package widget

import (
	"embed"
	"io/fs"

	"github.com/shibukawa/popcornweb/pw"
)

//go:embed assets migrations
var files embed.FS

const version = "v0.1.0"

func init() {
	pw.RegisterPackage(pw.Package{
		Module:        "example.com/widget",
		Version:       version,
		Assets:        mustSub(files, "assets"),
		Migrations:    mustSub(files, "migrations"),
		MigrationStem: "widget",
	})
}

func mustSub(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic("widget: " + err.Error())
	}
	return sub
}
```

Registration carries identity and bytes and nothing else. Middleware still goes
through `pw.RegisterExtension`, configuration through the generated binding that
already registers itself, and routes through an exported `Register` the
application calls. Nothing you register here answers a request on its own.

If the package's Go lives below the module root — a `ui/` directory, say —
`package.import` has to name that path, because it is what the consumer's
generated bootstrap imports:

```toml
[package]
module = "example.com/widget"
import = "example.com/widget/ui"
```

Leaving it out points the import at a path holding no Go, and the consumer's
build fails with the Go tool's `no required module provides package`, which
names neither the declaration nor this key. [`pw doctor`](/pw/project/doctor/)
reports it as `PW0144` before that happens.

### Generation runs here, once

`pw generate` in a package is the same run an application makes: it compiles
`.pw.html` into `_pw_gen.go` beside its source, using the same generator with
the same options. There is no package mode.

One consequence catches people. Generated code imports the template runtime
directly, so **the package's `go.mod` requires `tinybind-go` as a direct
dependency** once you have generated. Run `go mod tidy` after the first
generation and it lands there on its own — but it is a real part of your
published dependency set, not an implementation detail, because your committed
artifacts are what import it.

### Generated files are committed here

This is the one rule that inverts. In an application, `_pw_gen.go` is ignored by
git and recreated before every build. In a package it is **committed**, because
the consumer's build is `go build` and its generator never reads a dependency —
the module cache is read-only, and generation scope is a list of project-relative
directories by construction.

So a package repository carries no `**/*_pw_gen.go` ignore rule, and its release
gate is:

```sh
pw check
```

A tag that ships a stale artifact fails to compile in every project that
installs it. That is the loudest available failure and there is no repair path
for it, which is why the check belongs in CI rather than in a release checklist.

### Typed queries stay out

`generate.queries` must be empty in a package, and `pw generate` refuses a
non-empty one. A `.pw.sql` source compiles to one engine's placeholder syntax,
chosen when you publish; a package cannot know its consumer's engine. Shipping
one would produce a query that compiles everywhere and fails at the first call
in half the projects that install it. Write the query by hand, or take a
`*sql.DB` and let the application own it.

## The consuming side

```sh
pw add example.com/widget
```

The command writes the `go.mod` requirement and the `[[packages]]` entry, then
prints what is left. There is no wizard and no review screen, because nothing is
copied — the two lines it writes are the whole edit, and adding them by hand is
equally supported.

Why a declaration rather than just `go.mod`? Because `go.mod` says a module is
*available*, including every transitive dependency you never asked for, and the
declaration says you *intend to use it*. The generator needs the second fact: it
emits one blank import per declared package into the bootstrap file it already
maintains, and it cannot infer intent from a dependency graph. Removing the line
removes the import on the next generation.

A module that carries a package section and no declaration stays an ordinary Go
dependency. [`pw doctor`](/pw/project/doctor/) reports it — a transitive
dependency contributing assets and a schema is a surprise worth naming — but
nothing links it.

### Mind the order against `go mod tidy`

Nothing imports a freshly declared package until generation writes the import,
and `go mod tidy` removes a requirement that nothing imports. Run them in this
order:

```sh
go get example.com/widget   # or pw add, which does this for you
pw generate                 # writes the blank import
go mod tidy
```

Tidy first and the requirement disappears, so the next `pw generate` stops with
`packages "example.com/widget": not in the module graph`. `pw add` follows this
order on its own; writing the declaration by hand is where the trap is.

### The one line a declaration cannot replace

A package that serves routes exposes a `Register` function, and your entry point
calls it:

```go
mux := pw.NewServeMux()
widget.Register(mux)
```

This is deliberate. An installed route contributor would be a framework route
registration API, which the extension model does not have, and mounting is the
one contribution an application has an opinion about. `pw add` prints the call;
the manifest's `routes.register` is where it gets the name.

### Migrations arrive without being copied

Each package keeps its own migration stream, numbered independently of yours and
recorded in its own version table:

```sh
pw migrate up
```

Every declared package's stream applies before your own. The order among
packages comes from the Go module graph — a package cannot reference a table it
has never seen, and you can reference a package's tables, so the import
direction and the reference direction already agree. Nothing else is derived
from the graph.

Before anything runs, the pending versions are printed with the package that
owns them. That listing is doing real work: nothing landed in your repository,
so it is the only place you see what a dependency is about to do to your
database before it does it.

The trade is worth stating plainly. Copying migrations into the project would
put every statement under review in the repository that runs it. Keeping them in
the package means `go get -u` followed by `pw migrate up` is the entire upgrade,
with no second command anyone can forget. A copy that must be re-run after every
upgrade drifts silently, and silence about schema is worse than opacity about
it. If you want the statements, `pw migrate up` prints them and `go.sum` pins
the bytes.

Framework capabilities installed by `pw add auth` are unaffected: those are
written into your migration directory as before, because an operator ran a
wizard once and no module version moves underneath them.

### Configuration needs no section

A package's generated configuration binding registers its defaults when the
package is linked, so it works with nothing in your environment files. Add a
section only to override something. `pw add` names the section it registers; it
never writes one.

### Browser assets

Files the package embeds are served from a reserved, content-addressed path:

```
/_pw/pkg/<digest>/<name>
```

Identity is the digest of the bytes, so two packages shipping an identical file
serve one URL, a changed byte changes the URL, and the immutable cache header
stays honest. Nothing is written into your `public/` tree and nothing is read
from a filesystem at request time, which is what keeps this working on a TinyGo
target that has no filesystem at all.

To reference one from your document shell, ask for the URL:

```go
url, ok := pw.PackageAssetURL("example.com/widget", "widget.js")
```

This is the interim shape and it is honest about its limit: it does not scale
past a few packages, and it breaks quietly when an upgrade adds an asset you
never linked. A component declaring the asset it needs, with the framework
supplying the URL, is waiting on upstream work in
[`tinybind-go`](https://github.com/shibukawa/tinybind-go).

## Version skew is real

An application regenerates before every build, so its generator and its runtime
are one dependency at one version. A package freezes its artifacts at publish
time while `go.mod` picks the runtime at build time, and Go selects the higher of
the two — so the pair that runs was never quite the pair the author tested.

`package.generated_with` records what produced the artifacts. `pw doctor`
compares it against what your project resolves and reports a package generated
by a *newer* framework than yours, because its code may call a runtime entry
that does not exist yet. That is a compile error, and naming it in advance is
cheaper than reading it off the compiler.

## Pitfalls

**A stale committed artifact fails at `go build` in the consumer**, naming the
package. There is no repair path from the consuming side, and there should not
be: regenerating a dependency would republish it under a version its author
never tested.

**Two packages cannot share a migration stem.** Both registration and
`pw doctor` refuse it, because one shared version table would let each stream
read the other's applied versions as its own.

**`package.components.exported` is refused.** The key exists so the manifest has
a place for it when cross-module components land; writing it today is a load
error rather than a promise nothing can keep.

**A package downgrade is not a schema downgrade.** Going back to a version with
fewer migrations leaves applied versions with no source. That is reported, never
auto-reverted.

**The two readers of a stream must agree.** `pw migrate` reads the migration
files from the module directory, because it applies them without an application
binary and cannot reach another process's embedded copy; the in-process path and
`pw test` read the embed. They are the same files — unless your `//go:embed`
pattern misses one, which only your own release check will catch.

## Reference

Every key of both halves is in
[Build configuration](/reference/build-configuration/). The commands are
[`pw add`](/pw/project/add/), [`pw generate`](/pw/project/generate/),
[`pw migrate`](/pw/database/migrate/), and
[`pw doctor`](/pw/project/doctor/).
