---
title: pw build
description: Produce a release binary with generated code, minified CSS, and the built asset tree.
sidebar:
  order: 6
---

```sh
pw build
```

`pw build` turns the current project state into a release binary. It takes no
arguments; its inputs come from `popcornwave.toml` and the environment.

## What it does

1. runs [`pw generate`](/pw/project/generate/);
2. builds the Tailwind stylesheet **minified**, if Tailwind is enabled — this
   overrides `assets.tailwind.minify`, so a release is never accidentally
   unminified;
3. builds the [asset tree](/guides/frontend/static-assets/) into `dist/public`:
   it converts what the project asked to convert, writes the compressed `*.zstd`
   sidecars, and emits the manifest that decides every cache header;
4. rejects the build if `project.main` depends on a development-only package;
5. runs `go build` on `project.main` from `popcornwave.toml`.

The binary lands in the project root, named after the main package. The
scaffolded `.gitignore` already excludes it, along with everything under
`dist/` — the built tree, the conversion cache, and the manifest are all build
output.

Today, `contrib/devidp` is the only development-only package. It is the identity
provider used by [`pw dev`](/pw/project/dev/), and it signs users in without
checking a password. Linking that behavior into a deployable binary is a build
defect, not a production setting, so `pw build` stops and names the importing
package.

## Running the result

```sh
APP_ENV=prod ./myapp
```

`APP_ENV` selects which project-local configuration file is read; see
[Configuration](/guides/architecture/configuration/).

## Cross-compiling and TinyGo

`pw build` shells out to `go build`, so the usual environment variables apply:

```sh
GOOS=linux GOARCH=amd64 pw build
```

The generated path uses no runtime reflection, so the same sources can target
TinyGo. `pw build` always links with host `go`, so a TinyGo build runs the
preparation steps and then invokes that compiler itself:

```sh
pw prepare
tinygo build -scheduler=threads -o myapp ./cmd/myapp
```

[`pw prepare`](/pw/project/prepare/) is this command without its final step. Use
it rather than `pw generate`, which writes the generated Go but not
`dist/public` — a directory `public.go` names in a `go:embed` directive, so the
compiler fails on a tree that was never built.

`-scheduler=threads` is required for any engine that speaks a network protocol.
Under the cooperative scheduler a blocking socket call holds the whole runtime,
so a driver's cancellation watcher never runs and a query outlives its context
deadline without reporting one. The `database/postgres` and `database/mysql`
packages refuse to compile without the flag rather than letting that happen at
run time.

[Container Images](/guides/deployment/container-images/) uses both commands in
the two Dockerfiles `pw init` writes.

TinyGo's `net` package has no networking implementation of its own; every socket
passes through a Netdever registered by the program. Projects scaffolded with
TinyGo support include a root `tinygohelper.go` for that registration:

```go
//go:build tinygo

package publicassets

import _ "github.com/shibukawa/tinygodriver/netdev"
```

The `//go:build tinygo` constraint keeps the file out of host Go builds. Without
it a TinyGo binary compiles fine and then exits at startup:

```
2026/01/01 00:00:00 Netdev not set
```

Projects created with `--no-tinygo` do not get the file; add it by hand before
switching a project to TinyGo.

## In CI

Verify that generated code is current before building:

```sh
pw generate --check
pw build
```
