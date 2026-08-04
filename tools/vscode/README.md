# Popcorn Wave for Visual Studio Code

Syntax highlighting for the three Popcorn Wave source dialects:

| File | Language | Body |
| --- | --- | --- |
| `*.pw.html` | Popcorn Wave HTML | HTML, with slots and async boundaries |
| `*.pw.sql` | Popcorn Wave SQL | SQL of the project's configured engine |
| `*.pw.dynamo` | Popcorn Wave DynamoDB | a `table` and `key` clause list |

All three share one declaration header — `package`, `import`, `type`, `enum`,
`external`, and an `export`ed `component` or `statement` — and one set of
template forms inside the body: `{expression}`, `{if}` / `{else}` / `{/if}`,
`{for x in xs}` / `{/for}`, and `{await}` / `{fallback}` / `{recover}` /
`{/await}`, with `{{ }}` for a literal brace pair.

## What this version does

Highlighting, bracket matching, comment toggling, snippets, and **Format
Document**.

Formatting runs the tinybind formatter itself, compiled to WebAssembly with
TinyGo and bundled in the extension. No `pw` binary is spawned, nothing is
downloaded, and no network is touched, so the extension works on a file opened
with no workspace, no `popcornwave.toml`, and no `pw` on `PATH`, and is fully
supported in an untrusted workspace.

There is still **no language server, no diagnostics, and no completion**.
Those are planned for a later version through a `pw lsp` server. Until then,
run `pw generate --check` or `pw doctor` for the errors an editor would
otherwise report.

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

### Version skew

The bundled formatter is a fixed tinybind version (currently v0.3.2),
independent of the one your project pins. If they differ, this extension and
your CI can disagree about canonical form. A `pw fmt` delegation that removes
this is planned; this version always uses the bundled module and says so once
per session in the Popcorn Wave output channel.

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
never compiles it. `npm run check:wasm` rebuilds and fails when the result
differs from the committed artifact; CI runs it, pinned to the TinyGo version
in `wasm/TOOLCHAIN`.

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
