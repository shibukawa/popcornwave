# Popcorn Web for Visual Studio Code

Syntax highlighting for the three Popcorn Web source dialects:

| File | Language | Body |
| --- | --- | --- |
| `*.pw.html` | Popcorn Web HTML | HTML, with slots and async boundaries |
| `*.pw.sql` | Popcorn Web SQL | SQL of the project's configured engine |
| `*.pw.dynamo` | Popcorn Web DynamoDB | a `table` and `key` clause list |

All three share one declaration header — `package`, `import`, `type`, `enum`,
`external`, and an `export`ed `component` or `statement` — and one set of
template forms inside the body: `{expression}`, `{if}` / `{else}` / `{/if}`,
`{for x in xs}` / `{/for}`, and `{await}` / `{fallback}` / `{recover}` /
`{/await}`, with `{{ }}` for a literal brace pair.

## What this version does

Highlighting, bracket matching, comment toggling, snippets, **Format
Document**, and — when a `pw` binary is available — syntax diagnostics and a
document outline from a language server.

Formatting runs the tinybind formatter itself, compiled to WebAssembly with
TinyGo and bundled in the extension. No `pw` binary is spawned for it, nothing
is downloaded, and no network is touched, so highlighting and formatting work
on a file opened with no workspace, no `popcornweb.toml`, and no `pw` on
`PATH`, and they work in an untrusted workspace.

There is still **no completion, no go-to-definition, and no project-wide
check** in the editor. Run `pw check` or `pw doctor` for those.

## Diagnostics and the outline

A trusted workspace with `pw` available starts `pw lsp`, which reports a syntax
error as you type, fills the outline with the declarations of the open file,
answers **Go to Symbol in Workspace** across the project, and resolves hover and
go-to-definition on a declaration name. All of it comes from the same parsers
`pw generate` runs, so the editor and the build never disagree about whether a
file parses.

Hover, definition, references, completion, and inlay hints all resolve across
files and packages by the rule generation uses: your own package first, then the
exported declarations of what you import. Hover adds the Go types a component's
parameters lower to when the module analyzes cleanly. Completion is decided from
the text around the caret rather than from a parse, so it works in a buffer
mid-keystroke. Hints annotate what the source never writes — a `{val}` binding,
a loop variable, an `{await}` binding — and never a parameter, which writes its
own type.

What is resolved is the identifier under the cursor rather than the syntactic
position it sits in, so a name matching a declaration resolves inside a string
literal too.

**Rename Symbol** rewrites a declaration, every template reference to it, and
the handwritten Go that calls what it generated, in one edit set the editor
shows before applying. A generated file is not edited; `pw generate` writes the
new name itself. `pw rename` does the same with no editor.

Three more surfaces come from this server's own `pw/*` methods: **Popcorn Web
Routes** in the Explorer, **Peek Generated Code** on the context menu, and
**Preview Story**. `popcornweb.runtimeDiagnostics.enabled` additionally reports
what `pw dev` is currently failing on; it is the one thing the extension reads
over the network, it is a loopback read of a console you started, and it is off
by default.

The workspace search covers the `.pw.*` sources the project's `[generate]`
purposes list, and an open buffer overrides its indexed copy, so a declaration
you have not saved is still found. Editing `popcornweb.toml` reloads the model
in place; the client watches the file and the server registers no watcher of
its own.

The binary is looked up in this order, and never downloaded or installed:

1. the workspace devbox environment, at `.devbox/nix/profile/default/bin/pw`;
2. `PATH`;
3. `popcornweb.pw.path`, if you set it to an absolute path.

Nothing starts in an untrusted workspace, and nothing starts if you turn
`popcornweb.languageServer.enabled` off. In both cases highlighting and
formatting keep working, and the Popcorn Web output channel says once what was
not started and why. **Popcorn Web: Restart Language Server** restarts it after
you install or update `pw`.

The server reads the buffers the editor sends it and the `.pw.*` sources the
project declares. It writes no file and contacts nothing. It writes a protocol trace only if you point
`popcornweb.languageServer.log` at a file.

## Running pw

The palette carries **Generate**, **Check**, **Doctor**, **Migrate**, and
**Dev** under **Popcorn Web**, and all but Dev are also resolvable tasks:

```json
{ "type": "pw", "command": "check", "problemMatcher": ["$pw", "$pw-source"] }
```

`$pw` reads a `file:line:column: message`, which is what the template parsers
and the Go toolchain print; `$pw-source` reads a finding about a whole source,
such as a template no generate purpose compiles. Both are contributed here, so a
project writes neither.

`pw check` writes nothing and is the one to bind to a save; `pw generate` writes
files and is explicit only. `pw dev` gets one long-lived terminal rather than a
task, and a second invocation focuses it. `pw doctor` runs as `--format=json`
and its findings land on the files and configuration lines the report names,
with what the run could not determine printed to the output channel rather than
silently dropped. `pw migrate` asks first.

Nothing starts implicitly, and nothing runs in an untrusted workspace.

### Settings

| Setting | Default | Effect |
| --- | --- | --- |
| `popcornweb.languageServer.enabled` | `true` | Run `pw lsp`. |
| `popcornweb.pw.path` | `""` | Absolute path to `pw`, used when neither devbox nor `PATH` has one. |
| `popcornweb.languageServer.log` | `""` | File the server appends a protocol trace to. |

## Formatting

Use **Format Document**, or turn on `editor.formatOnSave` for these languages.
The extension does not enable format-on-save for you, but nothing stands in the
way of it: every `.pw.*` source in this repository formats and settles.

A source that does not parse is left exactly as it was, and a warning names the
line.

Formatting is safe to run repeatedly because `templatefmt` formats twice
internally and returns an error rather than a result that differs between the
passes. The extension relies on that rather than repeating the check, and pins
tinybind v0.3.2 or later to get it — `test/formatter.test.mjs` asserts the pin,
so a downgrade fails rather than quietly removing the protection.

### Which formatter ran

In a trusted workspace with a `pw` the extension can resolve, formatting runs
`pw fmt --stdin` — the same binary your build runs, so the editor and CI agree
about canonical form by construction. Everywhere else it runs the bundled
module: no workspace, no binary, an untrusted workspace, or a `pw` too old.

The bundled module is a fixed tinybind version (currently v0.5.17) and your
project's may differ, so which one produced a result is said once per session
in the Popcorn Web output channel.

A resolved `pw` is checked once before it is used, and the check is a property
rather than a version. The extension formats the source that was unstable
before tinybind v0.3.2 — a literal brace run in a `<style>` body — and requires
a second pass to return the first unchanged. A `pw` that fails it is refused,
because the extension applies a result without re-verifying it and relies on
the formatter's own idempotence guard to make that safe.

## Accuracy

The grammar is regular-expression based, so it approximates in the places a
parser would not:

- a declared parameter is not distinguished from any other identifier;
- a `{/if}` is not checked against its `{if}`;
- a `Type` and a `Component` name are both PascalCase and look alike.

It is built so the worst case is a missing color. It should never mark valid
source as an error, and where the real parser would refuse to read a brace —
inside a SQL string literal, a SQL comment, a dollar-quoted block, or a spaced
CSS or JavaScript block in `<style>` and `<script>` — the grammar does not read
one either.

## Development

The extension lives in the framework repository so a change to the template
syntax and the grammar that highlights it land together.

```bash
npm install
npm test
```

`npm test` needs no Go toolchain: the compiled formatter is committed. To
change the formatter entry, edit `wasm/main.go` and rebuild:

```bash
npm run build:wasm
```

`wasm/` is a Go module of its own, so `go build ./...` at the repository root
never compiles it. `npm run check:wasm` rebuilds the module and fails when the
committed artifact formats anything differently from that fresh build: every
`.pw.*` source in the repository, plus a handful of probes for the diagnostics
path, run through both. CI runs it, pinned to the TinyGo version in
`wasm/TOOLCHAIN`.

The comparison is behavioral because a byte comparison is not reproducible
across machines. The same TinyGo and the same Binaryen emit a different module
on macOS than on the Linux runner, and the Go patch level moves it again, so a
hash committed from a laptop can never match the one CI computes. The rebuild
writes to a scratch path and leaves the checkout alone.

The language server is not in this directory: it is `pw lsp`, in
`internal/pwlsp` of the framework repository, so the analysis is the same Go
the CLI runs. Its project model is read through the CLI's own
`popcornweb.toml` loader rather than a second reader, which is why
`internal/pwcli/lspproject.go` exists. What lives here is the client — `src/binary.js` decides which
`pw` to run, `src/client.js` decides what it is told to be, and both are
tested without an extension host.

`npm test` runs two things: the behavioral tests in `test/grammar.test.mjs`,
and a drift guard that tokenizes every `.pw.*` source in the repository and
compares it against `test/snapshots/tokens.txt`. A grammar edit that changes
what a real source looks like fails until the snapshot is refreshed:

```bash
UPDATE_SNAPSHOT=1 npm test
```

Review that diff — it is the whole point of the guard. An unexplained change
usually means a rule is matching more than it should.

Scope names are a contract, fixed by `rule:template-grammar-scopes` in
`.knowledge`. Renaming one breaks themes and the semantic token layer a future
language server will put on top, so treat a rename as a breaking change.

## Packaging

```bash
npm run package
```
