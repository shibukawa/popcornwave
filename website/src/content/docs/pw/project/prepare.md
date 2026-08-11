---
title: pw prepare
description: Run everything a build needs and stop before the compiler, for a build that something other than pw drives.
sidebar:
  order: 6.5
---

```sh
pw prepare [--debug] [--backend nethttp|fasthttp]
```

`pw prepare` is [`pw build`](/pw/project/build/) without its final step. It
produces no binary — it leaves a tree that a compiler can read, and stops.
`--backend` selects the build tags used by the dependency safety check. Provider
`--target` packaging belongs to `pw build` and is not accepted here.

## What it does

1. runs [`pw generate`](/pw/project/generate/);
2. builds the Tailwind stylesheet **minified**, if Tailwind is enabled;
3. builds the [asset tree](/guides/frontend/static-assets/) into `dist/public`,
   with its compressed sidecars and its manifest;
4. rejects the run if `project.main` depends on a development-only package.

That is the same list `pw build` performs before it links, in the same order,
because `pw build` is defined as this command plus the compiler. The two cannot
drift apart.

`--debug` keeps the source maps in the built tree, exactly as
[`pw build --debug`](/pw/project/build/#debug-artifacts) does. The other half of
that flag is not this command's to give: `-ldflags` belongs to the compiler line
you write, so a debug artifact from here is `pw prepare --debug` followed by a
`go build` you did not ask to strip.

Step 4 belongs here rather than beside the compiler for a reason worth saying
out loud: this command hands the tree to a compiler it does not run, so the
check that keeps `contrib/devidp` — an identity provider that signs users in
without checking a password — out of a deployable binary has to happen before
the handoff, not after it.

## When you need it

Reach for `pw build` unless something else owns the compile step. Three cases do.

**A TinyGo build.** `pw build` always links with host `go`, so a TinyGo project
prepares and then invokes its own compiler:

```sh
pw prepare
tinygo build -scheduler=threads -o myapp ./cmd/myapp
```

**A `go build` you want to control.** Cross-compiling to an unusual target,
passing `-ldflags`, or building several binaries out of one tree are all reasons
to write the compiler line yourself:

```sh
pw prepare
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/myapp ./cmd/myapp
```

**An image builder that owns `go build`.** ko and Cloud Native Buildpacks both
compile the project themselves and will not run generation for you. Run this
first, in the working tree, and invoke the builder second.

## `pw generate` is not enough

The natural guess is that `pw generate` already does this, since generated Go is
what the compiler is missing. It is not enough, and the way it fails is easy to
misread.

`pw generate` writes the `_pw_gen.go` files and nothing else. It does not build
`dist/public` — and `public.go` names that directory in a `go:embed` directive,
so a project prepared with `pw generate` alone fails to compile on a directory
that was never built. A project with Tailwind is also missing its stylesheet,
which fails later and more quietly: the pages render unstyled.

`pw generate` keeps its narrower job for the editor and for the `--check` gate
in CI. When you want a tree a compiler can consume, this is the command.

## In CI

Verify that generated code is current, then prepare and compile:

```sh
pw generate --check
pw prepare
go build ./cmd/myapp
```

[Container Images](/guides/deployment/container-images/) uses this command in
`Dockerfile.tinygo`, and explains why a Popcorn Wave build has a host phase at
all.
