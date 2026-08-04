---
id: concept:template-source-dialects
type: concept
title: Popcorn Wave Source Dialects
---
Three branded source kinds share one declaration header grammar and differ only in the body parser the declared output type selects; an editor that models the shared half models all three.

```yaml
owner: system:tinybind, which owns the grammar; Popcorn Wave owns only the file suffixes and the generation purposes
kinds:
  .pw.html:
    root_keyword: component
    output: html
    body: HTML, with slots and boundaries
    purpose: data:project-config generate.templates and generate.pages
  .pw.sql:
    root_keyword: statement
    output_prefix: sql
    body: SQL text of the data:project-config project.database dialect
    purpose: generate.queries
  .pw.dynamo:
    root_keyword: statement
    output_prefix: dynamo
    body: clause list rather than query text, per requirement:dynamodb-typed-queries
    purpose: generate.dynamo
    no_package_line: the directory is the package
shared_header:
  - optional package or module name, absent in .pw.dynamo
  - import "path" with an optional as alias
  - type or record declarations, enum declarations
  - external declarations, including external async and external live
  - annotations of the form @name(key: "value") preceding a declaration
  - an optional export modifier before the root keyword
  - a PascalCase declaration name, a typed parameter list, a colon, an output type, and a braced body
shared_body:
  expression: "{expr}" in a format-valid insertion context
  escape: "{{ ... }}" is literal text and carries no expression
  control:
    - "{if cond} ... {else if cond} ... {else} ... {/if}"
    - "{for item in collection} ... {/for}"
    - "{await name = Call(args)} ... {fallback} ... {recover err} ... {/await}"
  recursion: the shared parser owns the header of each control block and hands the body back to the active format parser
format_local:
  html: elements, attributes, <slot /> and named slots, raw-text elements
  sql: string literals, quoted identifiers, line and block comments, and dollar-quoted strings, none of which are template syntax
  dynamo: a required table clause and a key clause, in either order, separated by ";" on one line
generated_output: "{source-base}_pw_gen.go beside the source, excluded from version control per policy:generated-artifacts"
editor_consequence:
  - one grammar covers the shared header and the shared control forms, and three body grammars embed into it
  - the file suffix, not the content, decides which body grammar applies
  - a suffix outside its generate purpose is a project problem, not a syntax one, and belongs to requirement:editor-diagnostics
```
