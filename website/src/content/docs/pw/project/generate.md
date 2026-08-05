---
title: pw generate
description: Compile templates, SQL, and binding call sites into Go.
sidebar:
  order: 4
---

```sh
pw generate [--check]
```

Generation turns templates, SQL, page trees, and typed store declarations into
`_pw_gen.go` files **beside their sources**, then links the packages the
application needs. It prints only the paths that changed.

## Options

| Option | Effect |
| --- | --- |
| `--check` | write nothing; exit non-zero listing any stale file |

## What it reads

Generation is scoped **per purpose**. Each purpose lists the directories it may
read, and reads nothing else:

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
pages = ["pages"]
dynamo = []
firestore = ["entities"]
```

| Purpose | Reads | Generates |
| --- | --- | --- |
| `handlers` | route registrations, `pw.Parse` and response calls in Go | request binding, JSON codecs, the OpenAPI fragment |
| `templates` | `.pw.html` | typed renderers; also where the document shell and error pages are found |
| `queries` | `.pw.sql` | context-based query functions |
| `config` | `pw.RegisterConfig` and `pw.RegisterSubCommand` calls in Go | configuration and subcommand binding |
| `pages` | page-tree roots | route registration and page parameters for discovered routing |
| `dynamo` | `dynamo`-tagged Go types and `.pw.dynamo` | record codecs, keys, and typed DynamoDB queries |
| `firestore` | `firestore`-tagged Go types and `.pw.firestore` | entity codecs, keys, and typed Datastore-mode queries |

A directory may appear under several purposes — `handlers` usually appears under
both `handlers` and `templates`, because a page template lives beside the
handler that renders it. Each listed directory is walked recursively, so nested
packages need no entry of their own.

The original four keys—`handlers`, `templates`, `queries`, and `config`—are
required and have no default. `pages`, `dynamo`, and `firestore` are optional
for compatibility with older projects. An empty list is still the clearest way
to state that a project deliberately generates nothing for a purpose:

```toml
queries = []   # this project has no .pw.sql
```

That distinction matters: a forgotten key is an error, while `[]` is a decision
the next reader can see.

A source outside the purpose that owns it is reported and skipped rather than
failing the build, so deliberate samples and fixtures can live beside your code:

```
pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it
```

Go sources are never reported — ordinary Go code lives throughout a project, and
a call site outside its purpose simply gets no generated binding. A `_pw_gen.go`
left outside every purpose by an earlier layout **is** reported, since nothing
regenerates or removes it any more.

Besides declaration files, generation reads Go source for call sites:

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
[Templates](/guides/frontend/templates/).

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
