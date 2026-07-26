---
title: Installation
description: Install the pw command and add Popcorn Wave to a Go module.
sidebar:
  order: 1
---

Popcorn Wave requires **Go 1.26 or later**.

## The `pw` command

Almost everything — scaffolding, code generation, migrations, the development
server — goes through the `pw` command.

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

`pw init` writes a `go.mod` that already requires the framework, so a scaffolded
project needs no manual `go get`. To add Popcorn Wave to an existing module:

```sh
go get github.com/shibukawa/popcornwave
```

Application code imports the [`pw`](/guides/handlers/) package, which is the
stable application-facing API:

```go
import "github.com/shibukawa/popcornwave/pw"
```

## Devbox (optional)

Generated projects ship a `devbox.json` pinning Go and a Valkey service, and
`pw init --tailwind` adds the standalone Tailwind CSS binary to it. Devbox is
convenient but not required — if you already have Go on `PATH`, skip
`devbox shell` and run `pw dev` directly.

Install Devbox from [jetify.com/devbox](https://www.jetify.com/devbox/).

## Next steps

- [Getting started](/start/getting-started/) — create and run your first project.
