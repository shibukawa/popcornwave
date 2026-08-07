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
Usage: pw <command> [arguments]

Commands:
  init      create a project in a new directory
  add       enable a capability in a project that declined it
  new       scaffold a handler or a page beside the ones you have
  generate  regenerate everything derived from your sources
  migrate   inspect and apply database migrations
  seed      load seed datasets into the database
  build     generate, build assets, and compile the project
  dev       watch, regenerate, rebuild, and restart
  doctor    report what a named environment will actually run
  version   print the version, revision, and toolchain
  help      print this message
```

Install it with Homebrew, Nix, a release archive, or the Go toolchain — see
[Installation](/start/installation/).

## The commands

### Project

| Command | Purpose |
| --- | --- |
| [`pw init`](/pw/project/init/) | create a runnable project |
| [`pw add`](/pw/project/add/) | install a capability the project declined at init |
| [`pw new`](/pw/project/new/) | scaffold one more handler, route, and template |
| [`pw generate`](/pw/project/generate/) | compile `.pw.html` and `.pw.sql` into Go |
| [`pw dev`](/pw/project/dev/) | watch, regenerate, migrate, and restart |
| [`pw prepare`](/pw/project/prepare/) | run everything a build needs, stopping before the compiler |
| [`pw build`](/pw/project/build/) | produce a release binary |
| [`pw doctor`](/pw/project/doctor/) | report what an environment would run, and what is wrong |

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
[Custom Commands](/guides/architecture/custom-commands/) for that boundary.
