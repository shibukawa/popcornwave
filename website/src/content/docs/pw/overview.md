---
title: Overview
description: What the pw development tool does, and where each command fits.
---

One command connects the project lifecycle. `pw` scaffolds the first files,
compiles templates and SQL into Go, runs the development loop, manages the
database, and produces release builds.

```sh
pw help
```

```
Usage: pw <command>
Commands: init, add, new, generate, migrate, seed, build, dev
Migrate actions: status, version, up, up-by-one, up-to, down, down-to, create, validate, snapshot
Seed usage: pw seed [--dir=testdata/seed] [name...]
```

Install it with:

```sh
go install github.com/shibukawa/popcornwave/cmd/pw@latest
```

## The commands

### Project

| Command | Purpose |
| --- | --- |
| [`pw init`](/pw/project/init/) | create a runnable project |
| [`pw add`](/pw/project/add/) | install a capability the project declined at init |
| [`pw new`](/pw/project/new/) | scaffold one more handler, route, and template |
| [`pw generate`](/pw/project/generate/) | compile `.pw.html` and `.pw.sql` into Go |
| [`pw dev`](/pw/project/dev/) | watch, regenerate, migrate, and restart |
| [`pw build`](/pw/project/build/) | produce a release binary |

### Database

| Command | Purpose |
| --- | --- |
| [`pw migrate`](/pw/database/migrate/) | inspect, apply, and roll back migrations |
| [`pw seed`](/pw/database/seed/) | load seed datasets |

## Finding the project

Every command except `pw init` needs a project, but it does not need to run at
the project root. `pw` walks upward from the working directory until it finds
`popcornwave.toml`, allowing subcommands to work from any nested directory. If
the search reaches the top without that file, the command fails with
`popcornwave.toml not found`.

## Exit status

`0` on success, `1` on a command failure, `2` when no command was given. Errors
are written to standard error prefixed with `pw:`.

## Not to be confused with

The deployed binary has a different command line: configuration flags,
configuration scaffold output, and application-defined subcommands. See
[Application CLI](/guides/application-cli/) for that boundary.
