---
title: Installation
description: Install the pw command and add Popcorn Wave to a Go module.
sidebar:
  order: 1
---

Popcorn Wave requires **Go 1.26 or later**. From there, the only required setup
is the `pw` command and the library dependency it manages.

## The `pw` command

Scaffolding, code generation, migrations, and the development server all go
through `pw`, so install it first:

```sh
go install github.com/shibukawa/popcornwave/cmd/pw@latest
```

```sh
pw help
```

```
Usage: pw <command>
Commands: init, generate, migrate, seed, build, dev
Migrate actions: status, version, up, up-by-one, up-to, down, down-to, create, validate, snapshot
Seed usage: pw seed [--dir=testdata/seed] [name...]
```

## The library

For a new project, `pw init` writes a `go.mod` that already requires the
framework; no manual `go get` is needed. An existing module needs one additional
step:

```sh
go get github.com/shibukawa/popcornwave
```

Application code imports the [`pw`](/guides/frontend/handlers/) package, which is the
stable application-facing API:

```go
import "github.com/shibukawa/popcornwave/pw"
```

## Devbox (optional)

Generated projects include a `devbox.json` that pins Go and a Valkey service.
When you use `pw init --tailwind`, it pins the standalone Tailwind CSS binary as
well. Devbox keeps those tools reproducible, but it is optional: if Go is
already on `PATH`, skip `devbox shell` and run `pw dev` directly.

Install Devbox from [jetify.com/devbox](https://www.jetify.com/devbox/).

## Next steps

- [Getting started](/start/getting-started/) — create and run your first project.
