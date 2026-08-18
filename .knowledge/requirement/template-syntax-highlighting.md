---
id: requirement:template-syntax-highlighting
type: requirement
title: Template Syntax Highlighting
---
A .pw.html, .pw.sql, or .pw.dynamo file highlights its declaration header, its template expressions, and its embedded HTML, SQL, or clause body, with no server and no project.

```yaml
status: implemented at tools/vscode, version 0.1.0
stage: 1 of vision:editor-support
source: concept:template-source-dialects
mechanism: decision:textmate-grammar-first
scopes: rule:template-grammar-scopes
registration: requirement:editor-language-registration
behavior:
  - the header colors keywords, the declaration name, parameter names, primitive types, and the output type
  - an annotation line colors its name and its string argument values
  - a body region colors as its embedded language, and every "{" expression reopens the template scope inside it
  - a control form colors its keyword and its braces, and its body keeps the embedded language
  - "a {{ ... }} region colors as literal text and never as an expression"
  - an SQL literal, quoted identifier, comment, or dollar-quoted region stays inside the SQL scope
  - script and style content follows the tinybind raw-text insertion gate, so a spaced CSS block brace and a "${}" placeholder stay authored code while a tight call shape stays an insertion
  - a dynamo body colors table and key as clauses, with parameters as expressions and attribute names as plain identifiers
bracket_and_comment:
  source: the language configuration of requirement:editor-language-registration and the embeddedLanguages map
  effect: comment toggling and bracket matching follow the region the cursor is in
non_goals:
  - distinguishing a declared parameter from any other identifier, which needs resolution
  - marking an unbalanced or unknown construct as an error, per decision:textmate-grammar-first
  - highlighting a generated *_pw_gen.go file as anything but Go
  - a Popcorn Web color theme
acceptance:
  - opening each of the repository's own .pw.html, .pw.sql, and .pw.dynamo sources produces the token sequence its fixture records
  - a file opened with no workspace, no popcornweb.toml, and no pw binary highlights identically
  - "a SELECT containing '{' inside a string literal is not tokenized as an expression"
  - an HTML attribute value containing an expression colors the expression and keeps the attribute scope around it
  - a dynamo body carries no source.sql scope anywhere
  - an unterminated body degrades to unscoped text and reports nothing
verification:
  behavioral: one test per acceptance line, run against the same TextMate engine system:vscode runs
  corpus: the drift_guard snapshot of rule:template-grammar-scopes
  not_covered: the visual result in a real editor window, which no automated check here asserts
```
