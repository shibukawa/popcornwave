---
title: Editor Support
description: Highlighting and formatting for the template dialects with no setup at all, and syntax diagnostics from the same parsers the build runs.
sidebar:
  order: 11
---

A `.pw.html` file opened in an editor that knows nothing about it is one
undifferentiated block: the declaration header, the `{}` expressions, and the
HTML body all render as plain text. That is the ordinary state of a new source
language, and it costs more than it looks like it should — the mistake you make
in a template is usually a brace or a closing form, and both are exactly what
color would have shown you.

The Popcorn Web extension for Visual Studio Code covers all three dialects.
Install it from the Marketplace or Open VSX, search for **Popcorn Web**.

## What works with nothing installed

Highlighting, bracket matching, comment toggling, snippets, and **Format
Document** need no `pw` binary, no `popcornweb.toml`, and no network. The
formatter is the framework's own, compiled to WebAssembly and shipped inside the
extension, so it runs on a file you opened from a review checkout with no
project around it — and in a workspace you have not trusted.

That property is deliberate. Reviewing a pull request in a browser-hosted editor
is the case where a template most needs to be readable and where the least
tooling is available.

Where a `pw` *is* available and the workspace is trusted, formatting runs
`pw fmt --stdin` instead — the same binary your build runs, so the editor and CI
cannot disagree about canonical form. The bundled module is a fixed tinybind
version and your project's may differ, so the output channel says once per
session which one produced a result.

A resolved `pw` is checked once before it is trusted, and the check is a
property rather than a version: the extension formats the source that was
unstable before tinybind v0.3.2 — a literal brace run in a `<style>` body — and
requires a second pass to return the first unchanged. One that fails is refused,
because the editor applies a formatting result without re-verifying it and
relies on the formatter's own guard to make that safe.

```bash
# Optional: format on save, for these languages only.
# "[pw-html]": { "editor.formatOnSave": true }
```

A source that does not parse is left exactly as it was, and a warning names the
line. Formatting never half-applies.

## What `pw lsp` adds

With a `pw` binary available in a trusted workspace, the extension starts
`pw lsp`. You get a syntax error reported as you type, an outline of the
declarations in the open file, **Go to Symbol in Workspace** across the project,
and hover and go-to-definition on any declaration name.

All of it comes from the parsers `pw generate` runs. There is no second parser written
in TypeScript, which is why the editor and the build cannot disagree about
whether a file parses — the alternative drifts on every framework release, and
for the one feature where a byte difference is the whole answer.

### Hover, navigation, completion, and hints

Put the cursor on a name and hover shows what it is: the declaration as its
source spells it, a record's fields, and — for a component whose module analyzes
cleanly — the Go types its parameters lower to, which is the one thing the
template never states. Go-to-definition jumps to it, across files and across
packages, following the same rule generation follows: a declaration in your own
package wins, and an import brings in that package's exported declarations.

This works with no generated output at all.

Navigation crosses into Go as well. Find-references on a declaration lists the
handwritten Go that calls what it generated, alongside the template references —
which is the crossing this framework's indirection is made of, and the one a
result list stopping at the template boundary would hide. The names it looks for
are read out of the generated file rather than derived from the declaration,
because the naming scheme belongs to the generator and a copy of it here would
drift on the next release.

From the other side, **Popcorn Web: Go to Template Declaration** on a symbol in
a `.go` file opens the `.pw.*` declaration that produced it, skipping the
`_pw_gen.go` in between. It is a command rather than a go-to-definition
provider, because gopls owns Go files and standing in front of it would be
worse than adding one entry to a menu.

Find-references works the same way in the other direction, and it only scans the
files that can actually see the declaration, so another package's unrelated
`Card` is not reported as a use of yours.

Completion offers what the position can hold: root keywords and the output types
your dialect allows in a header, the primitives and your declared records after a
`:`, the components you can reach after a `<` — inserted with the parameters they
require — and, inside `{ }`, the parameters and bindings in scope plus the
control forms, each with its closing form so accepting one cannot leave a body
half-written. With no project loaded you still get the keywords and the
primitives, and nothing that would need resolution.

Inlay hints annotate what the source never writes: the type a `{val}` binding
holds, the element type a loop binds, and the type an `{await}` binding settles
to. A parameter is not annotated, because it writes its own type. A binding whose
expression this server does not evaluate gets no hint rather than a guessed one.
The families are separately switchable.

One honest limit runs through all of it: what gets resolved is the identifier
under the cursor, not the position it was written in. A word that matches a
declaration name resolves even inside a SQL string literal. Telling those apart
needs the body syntax tree rather than the name graph.

### What the workspace search covers

The server reads `popcornweb.toml` and indexes the `.pw.*` sources your
`[generate]` purposes list — nothing else. A template in a directory no purpose
names is invisible to the search for the same reason it is invisible to
`pw generate`: a purpose reads only the directories it lists, and a search that
found more than generation does would be telling you about a file the build will
never compile. That file still gets syntax diagnostics; it is only absent from
the project's answers.

A declaration you have just typed and not yet saved is found. The index holds
what is on disk, and the buffers the editor has open override it, so the result
lands where the declaration is now rather than where it was at the last save.

Editing `popcornweb.toml` reloads the model in place — adding a directory to
`generate.templates` makes its declarations searchable without restarting
anything. If the file will not load, the error is reported on
`popcornweb.toml` itself, and highlighting, formatting, and syntax diagnostics
carry on. Open with no `popcornweb.toml` above you at all and the server says so
once, then serves syntax diagnostics only; nothing that needs a project is
guessed at.

You do not run `pw lsp` yourself. It speaks the Language Server Protocol on
stdin and stdout, so anything you write to that stream ends the session; the
editor starts it, and the command exists so that editors other than VS Code can
start it too.

The binary is looked up in this order, and never downloaded or installed:

1. the workspace devbox environment, at `.devbox/nix/profile/default/bin/pw`;
2. `PATH`;
3. `popcornweb.pw.path`, when you set it to an absolute path.

Devbox comes first because a project that pins its toolchain wants the editor
running the version the project pins, not whichever one is on your `PATH`. That
is also why the configured path is last: it is the answer for a machine where
neither of the first two has one, rather than an override that would quietly put
your editor on a different `pw` from your build.

### When it does not start

In an untrusted workspace, nothing starts — a workspace-relative binary path is
workspace-controlled input, and opening a file must never run a project's own
tooling. With no `pw` anywhere, nothing starts either. In both cases
highlighting and formatting keep working, and the **Popcorn Web** output channel
says once what was not started and why.

After installing or upgrading `pw`, run **Popcorn Web: Restart Language Server**
rather than reloading the window.

| Setting | Default | Effect |
| --- | --- | --- |
| `popcornweb.languageServer.enabled` | `true` | Run `pw lsp` at all. |
| `popcornweb.pw.path` | `""` | Absolute path to `pw`, used only when devbox and `PATH` have none. |
| `popcornweb.languageServer.log` | `""` | File the server appends a protocol trace to. Empty means it writes nothing. |

The server reads only the buffers the editor sends it, writes no file in your
workspace, and contacts nothing.

### The routes, the generated code, and the storybook

Three things sit outside what LSP has a method for, so the extension asks the
server for them directly.

**Popcorn Web Routes** in the Explorer lists every route your page trees serve,
with the page template, its layout chain, and whether it has a `page.go` of its
own. Selecting one opens its template. The view says what it does not cover:
routes registered in Go are calls rather than directories, and finding them
needs the resolved import graph, which this does not load.

**Peek Generated Code**, on the context menu of a `.pw.*` file, opens the Go a
declaration produced in a read-only view beside it. Nothing is generated by
opening it — generation writes files, and that stays an explicit action — so
before you have run `pw generate` the view says so and tells you what to run. If
the source has changed since the last generation, the view is labelled stale
rather than passed off as current.

**Preview Story** opens the [storybook](/productivity/dev-console/) pane at the
component under the cursor. The extension computes which URL that is and opens
it; it renders nothing itself.

### Failures from the running loop

`pw dev` finds failures a parser cannot: a generation error, a build error, a
template that only breaks when it renders. Switch on
`popcornweb.runtimeDiagnostics.enabled` and those appear in the Problems view at
the position the loop named, marked `pw dev` so they are never confused with what
`pw generate` would report.

This is the one thing the extension reads over the network, and it is a loopback
request to the development console you started. It is off by default, it contacts
nothing while it is off, and nothing leaves the machine. A finding is cleared
when the loop rebuilds, because a finding that outlives its build describes code
that may no longer exist.

## Running pw from the editor

The commands you run while editing are in the palette under **Popcorn Web**, and
they are the same commands with the same arguments you would type:

| Command | Runs | Notes |
| --- | --- | --- |
| Generate | `pw generate --code-only` | Writes the generated Go a diagnostic points into. The asset tree is a build's concern, not a keystroke's. |
| Check | `pw check` | Reports generated Go that is stale or missing. Writes nothing. |
| Doctor | `pw doctor --format=json` | Findings appear on the files and configuration lines they name. |
| Migrate | `pw migrate up` | Asks first. |
| Dev | `pw dev` | A terminal of its own, one at a time. |

Each of them is also a task, so a project composes them in `tasks.json` without
writing a problem matcher:

```json
{
  "version": "2.0.0",
  "tasks": [
    { "type": "pw", "command": "check", "problemMatcher": ["$pw", "$pw-source"] }
  ]
}
```

`pw check` is the one to bind to a save. It writes nothing, and it reports the
generation errors the language server does not — an unknown external function,
a result type a statement cannot satisfy — because those need the whole project
analyzed rather than one buffer parsed. Bind `Generate` to a keystroke instead
of a save: it writes files.

`pw dev` gets a terminal rather than a task, because the loop owns services, the
identity provider, and the telemetry viewer, and its output is something you
watch rather than something a problem matcher reads. The URLs it prints are
clickable because the terminal makes them clickable; the extension embeds no
viewer of its own. Running the command a second time focuses the terminal
already running rather than starting a second loop.

`pw migrate` asks before it runs. Migrations are forward-only against a real
database, and a command in a palette is one keystroke away from a click you did
not mean.

Nothing here starts on its own. No save runs `pw generate`, no open starts
`pw dev`, and an untrusted workspace runs none of it.

## What it does not do yet

There is no completion, no find-references, and no rename. A missing external
function or a result type a statement cannot satisfy is not reported as you
type: those need the whole project analyzed, which is what the `pw check` task
above is for.

One thing it does report from the project model: a `.pw.*` source no `[generate]`
purpose compiles, in the same words `pw generate` uses for it — with a quick fix
that lists the file's own directory under the purpose that would compile it, as
a one-line edit to `popcornweb.toml`. It does not offer to move the file: where
that belongs is a judgement about your layout, and listing the directory you
already chose is not. A template inside
a page tree that is not `page.pw.html`, `layout.pw.html`, or
`document.pw.html` gets the same treatment, because a tree compiles only the
names it reserves.

So: reach for the editor for the mistakes you make while typing, and keep
`pw check` in your build for everything that needs the whole project in view.
If your editing loop is already `pw dev` in a terminal beside the editor, the
extension adds color and canonical formatting and changes nothing else about it.

## Other editors

The analysis is a `pw` subcommand rather than an extension feature, so any
editor that can start a language server reaches the same diagnostics, outline,
hover, definition, references, completion, and inlay hints with a thin client
and no Popcorn Web code of its own.

Three things configure it: the command is `pw lsp --stdio`, the file types are
`.pw.html`, `.pw.sql`, and `.pw.dynamo`, and the root marker is the nearest
`popcornweb.toml`.

In Neovim, with `nvim-lspconfig`:

```lua
vim.filetype.add({ pattern = {
  [".*%.pw%.html"] = "pw-html",
  [".*%.pw%.sql"] = "pw-sql",
  [".*%.pw%.dynamo"] = "pw-dynamo",
} })

vim.lsp.config("pw", {
  cmd = { "pw", "lsp", "--stdio" },
  filetypes = { "pw-html", "pw-sql", "pw-dynamo" },
  root_markers = { "popcornweb.toml" },
})
vim.lsp.enable("pw")
```

In Zed, an extension declares the same three things in its `extension.toml`:

```toml
[language_servers.pw]
name = "Popcorn Web"
languages = ["Popcorn Web HTML", "Popcorn Web SQL", "Popcorn Web DynamoDB"]
```

with the server started as `pw lsp --stdio` from the extension's Rust entry
point. Highlighting is the one part that does not carry over: Zed reads
tree-sitter grammars and this repository ships a TextMate one, so an editor in
that family either contributes a grammar of its own or relies on the semantic
tokens the server sends, which colour only while the server is running.

Two of this server's methods are its own rather than LSP's, and a client that
does not know them simply never sends one: `pw/routes` lists the page tree,
`pw/generatedFor` returns the Go a declaration produced, `pw/storyFor` gives
the storybook URL of a component, and `pw/project` reports the loaded project.
