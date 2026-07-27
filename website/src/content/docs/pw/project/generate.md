---
title: pw generate
description: Compile templates, SQL, and binding call sites into Go.
sidebar:
  order: 2
---

```sh
pw generate [--check]
```

Generation turns every `.pw.html` and `.pw.sql` source into a `_pw_gen.go` file
**beside it**, then links the document registration package into the main
package. It prints only the paths that changed.

## Options

| Option | Effect |
| --- | --- |
| `--check` | write nothing; exit non-zero listing any stale file |

## What it reads

Generation walks the whole project tree and processes **every directory
containing** a `.go`, `.pw.html`, or `.pw.sql` file, skipping `.git`, `vendor`,
`node_modules`, and `.devbox`. There is no package list to maintain — creating a
directory is enough.

Besides the template files, it reads your Go source for call sites:

| Call | Generates |
| --- | --- |
| `pw.Parse[T]` | request binding for `T` |
| `pw.WriteAPI[T]` | JSON encoding for `T` |
| `pw.NewStream[T]` | stream encoding for `T` |
| `pw.RegisterConfig[T]` | configuration binding for `T` |
| `pw.RegisterSubCommand[T]` | subcommand parsing for `T` |
| `pw.BadRequest` and the other error constructors | the documented error responses |

The same evidence feeds one OpenAPI 3.1 fragment per package. Those fragments
are merged deterministically at build time, so the API description follows the
code rather than a separate annotation set.

## What it writes

Generated Go is build output, not source:

- filenames are `{source-base}_pw_gen.go`, always beside the source;
- they are excluded from version control by the scaffolded `.gitignore`;
- `.vscode/settings.json` hides them from the editor;
- they are recreated on every application build.

They can always be reproduced. Do not edit or commit them.

`cmd/<name>/popcornwave_bootstrap_pw_gen.go` is the exception in kind rather
than in rule: it is a generated file of blank imports that links the document
shell and the embedded public assets into the binary, so no handler has to
reference them. It is removed automatically when neither exists.

## The single-document rule

A project has exactly one `document.pw.html`. If generation finds more anywhere
in the tree it fails with:

```
pw: multiple default documents: templates/document.pw.html, admin/document.pw.html
```

Alternative shells are ordinary exported components with an unnamed slot,
selected per handler with `pw.WriteHTMLChain`. See
[Templates](/guides/templates/).

## In CI

```sh
pw generate --check
```

Because Git ignores generated Go, a repository diff cannot reveal stale output
in CI. `--check` regenerates in memory and fails if any file would change:

```
pw: generated files are stale:
  handlers/home_pw_gen.go
```

Both [`pw dev`](/pw/project/dev/) and [`pw build`](/pw/project/build/) generate
first. Direct invocation is therefore mostly useful for CI and for diagnosing
generation errors.
