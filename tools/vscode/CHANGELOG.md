# Changelog

## 0.3.0

The bundled formatter moves to tinybind v0.3.2, which fixes both defects 0.2.0
had to work around.

- A literal brace run inside a `<script>` or `<style>` body is now written back
  as authored. It previously gained a brace pair on every pass, and the one
  file in this repository that hit it was refused rather than formatted. Every
  `.pw.*` source in the repository now formats and settles.
- `ON CONFLICT ... DO UPDATE SET` stays on one line instead of splitting across
  three.
- The extension no longer double-formats to verify the result. `templatefmt`
  now performs that check itself, closer to the AST than the extension can, and
  returns an error rather than an unstable result. The extension pins v0.3.2 or
  later to get it, and a test asserts the pin so a downgrade cannot silently
  remove the protection.
- Format-on-save is still not enabled for you, but it is no longer discouraged.

## 0.2.0

Formatting, with no external dependency.

- **Format Document** for all three dialects, running the tinybind v0.3.1
  formatter compiled to WebAssembly with TinyGo and bundled in the extension.
  No process is spawned, nothing is downloaded, and no network is touched, so
  formatting works with no workspace, no `pw` on `PATH`, and in an untrusted
  workspace.
- Every result is verified before it is applied. The extension formats, formats
  the result again, and edits the buffer only when both parse and agree. A
  source that fails either check is left untouched and reported with its line.
  This catches a live upstream defect where a literal brace run inside a
  `<script>` or `<style>` body gains a brace pair on every pass.
- An already canonical buffer produces no edit, so it never enters the undo
  stack.
- Format-on-save is not enabled or recommended yet; the defect above still
  fires on real sources.
- Fixed: the DynamoDB grammar colored `limit`, `index`, `consistent`,
  `forward`, and `backward` as clause keywords. The body grammar has only
  `table`, `key`, and the `filter` it rejects by name.

## 0.1.0

First release. Highlighting only; no language server and no diagnostics.

- `pw-html`, `pw-sql`, and `pw-dynamo` languages for `*.pw.html`, `*.pw.sql`,
  and `*.pw.dynamo`.
- One shared grammar for the declaration header and the template forms, with
  the HTML, SQL, and DynamoDB clause bodies embedded into it.
- Template expressions are highlighted inside HTML text, quoted attribute
  values, and unquoted attribute values.
- A brace inside a SQL string literal, quoted identifier, comment, or
  dollar-quoted block stays SQL.
- Inside `<script>` and `<style>`, only the tight insertion shapes open a
  template expression; a spaced block brace and a `${}` placeholder stay
  authored JavaScript or CSS.
- Bracket matching, comment toggling, and indentation per dialect.
- Snippets for declarations, control blocks, and DynamoDB key clauses.
