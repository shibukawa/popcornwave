---
title: pw build
description: Produce a release binary with generated code, minified CSS, and prepared assets.
sidebar:
  order: 4
---

```sh
pw build
```

Builds the release binary. It takes no arguments.

## What it does

1. runs [`pw generate`](/pw/project/generate/);
2. builds the Tailwind stylesheet **minified**, if Tailwind is enabled — this
   overrides `assets.tailwind.minify`, so a release is never accidentally
   unminified;
3. prepares the public asset tree, writing the compressed `*.zstd` sidecars
   that the asset middleware serves to clients accepting them;
4. rejects the build if `project.main` depends on a development-only package;
5. runs `go build` on `project.main` from `popcornwave.toml`.

The binary lands in the project root, named after the main package. The
scaffolded `.gitignore` already excludes it, along with `public/**/*.zstd`.

The only development-only package today is `contrib/devidp`, the identity
provider [`pw dev`](/pw/project/dev/) runs. It signs anyone in without checking
a password, so linking it into a deployable binary is a defect rather than a
configuration mistake, and the build stops with the importing package named.

## Running the result

```sh
APP_ENV=prod ./myapp
```

`APP_ENV` selects which project-local configuration file is read; see
[Configuration](/guides/configuration/).

## Cross-compiling and TinyGo

`pw build` shells out to `go build`, so the usual environment variables apply:

```sh
GOOS=linux GOARCH=amd64 pw build
```

Because nothing in the generated code path uses runtime reflection, the same
sources also target TinyGo. Drive that compiler directly after generating:

```sh
pw generate
tinygo build -o myapp ./cmd/myapp
```

## In CI

Verify that generated code is current before building:

```sh
pw generate --check
pw build
```
