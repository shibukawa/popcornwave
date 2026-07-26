---
title: Overview
description: What the pw development tool does, and where each command fits.
---

`pw` is the development tool: it scaffolds projects, compiles templates and SQL
into Go, runs the development loop, manages the database, and builds releases.

```sh
pw help
```

```
Usage: pw <command>
Commands: init, generate, migrate, seed, build, dev
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
| [`pw generate`](/pw/project/generate/) | compile `.pw.html` and `.pw.sql` into Go |
| [`pw dev`](/pw/project/dev/) | watch, regenerate, migrate, and restart |
| [`pw build`](/pw/project/build/) | produce a release binary |

### Database

| Command | Purpose |
| --- | --- |
| [`pw migrate`](/pw/database/migrate/) | inspect, apply, and roll back migrations |
| [`pw seed`](/pw/database/seed/) | load seed datasets |

## Finding the project

Every command except `pw init` runs from inside a project. `pw` locates the
root by walking up from the working directory until it finds
`popcornwave.toml`, so subcommands work from any subdirectory. If there is no
such file, the command fails with `popcornwave.toml not found`.

## Exit status

`0` on success, `1` on a command failure, `2` when no command was given. Errors
are written to standard error prefixed with `pw:`.

## Not to be confused with

The binary you build has its own command line — configuration flags, scaffold
output, and any subcommands you define. That is covered in
[Application CLI](/guides/application-cli/).
