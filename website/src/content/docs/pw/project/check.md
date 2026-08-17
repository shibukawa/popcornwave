---
title: pw check
description: Report generated Go that is stale or missing, without writing anything.
sidebar:
  order: 4.5
---

```sh
pw check
```

`pw check` plans everything [`pw generate`](/pw/project/generate/) would write
into `_pw_gen.go` files, compares it against what is on disk, and writes nothing.
It takes no flags.

Generated Go is excluded from version control, so a repository diff cannot show
that it has gone stale. This command is what stands in for that diff:

```
pw: generated files are stale:
  handlers/home_pw_gen.go
```

It exits non-zero on that, and zero when every file matches its sources. A file
that is missing entirely counts as stale — which is what a clean checkout looks
like, since nothing committed the output.

## What it does not check

Only the generated Go. The stylesheet and the asset tree that
[`pw generate`](/pw/project/generate/) also writes are excluded from version
control too, and for those there is nothing committed to compare against at all.

So this command verifies less than `pw generate` writes, and the difference
matters when you read a green CI run: it means the generated Go matches its
sources, **not** that the tree compiles. `go build` is still what answers that.

Two neighbouring commands answer the adjacent questions, and none of the three
runs the others:

| Command | Answers |
| --- | --- |
| `pw check` | generated Go that no longer matches its sources |
| `pw fmt --check` | template and query sources that are not in canonical form |
| [`pw doctor`](/pw/project/doctor/) | configuration, wiring, and what a named environment will actually run |

## In CI

```sh
pw fmt --check
pw check
pw build
```

`pw fmt` rewrites sources, so it comes first: a formatting pass after generation
would leave the generated output describing the previous text.

`pw doctor` runs this check itself before reporting the configuration sections it
reads out of generated metadata, and downgrades those sections when it fails —
there is no point describing a configuration surface from a stale description of
it.

## In a package project

A [component package](/guides/deployment/package/) commits its generated Go,
because a consumer builds with `go build` alone and regenerates nothing. Nothing
in the consumer's project can detect a stale artifact beyond a compile error
naming your package, so this command is the release gate, and `pw init --kind
package` scaffolds a workflow that runs it.
