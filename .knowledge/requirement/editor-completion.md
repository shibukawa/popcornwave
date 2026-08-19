---
id: requirement:editor-completion
type: requirement
title: Template Completion
---
Completion offers what the current position can legally hold, which for these dialects is a small closed set the server already knows exactly.

```yaml
status: implemented at internal/pwlsp; the position is decided from the text around the caret rather than the body AST, so completion works in a buffer mid-keystroke that does not parse
stage: 3 of vision:editor-support
server: requirement:pw-language-server
positions:
  header:
    - root keywords and modifiers valid at the start of a declaration
    - primitive and declared type names in a parameter list
    - the output types the root keyword allows, per concept:template-source-dialects
    - an import path from the module's own package list
  expression:
    - parameters, record fields, and loop variables in scope
    - declared and external function names, with their signatures
    - control forms, offered with their closing form
  html_body:
    - component references declared in the file or reachable through an import, with their parameters
    - slot names a slot-capable component declares
    - the closing tag of an open element, which the embedded HTML grammar cannot resolve across a control form
  sql_body:
    - column and table names from data:migration-source, when the migration directory parses
    - the parameters of the enclosing statement inside a "{" expression
  dynamo_body:
    - table names from decision:dynamodb-table-registry
    - attribute names from the result type's dynamo tags, filtered to key attributes inside a key clause
    - the sort key predicates requirement:dynamodb-typed-queries allows
snippets:
  - a component, statement, or dynamo declaration skeleton
  - an if, for, and await block with its closing form
  - contributed as language snippets, so they exist from stage 1 even though resolution does not
non_goals:
  - HTML element and attribute completion, which the embedded language already provides
  - general SQL keyword completion, which an installed SQL extension provides for the embedded region
  - completion in a generated file
acceptance:
  - completion inside a key clause offers only key attributes of the bound type
  - completion inside an expression offers the enclosing declaration's parameters and no unrelated identifier
  - a component completion inserts its required parameters
  - completion with no loaded project offers snippets and keywords only, and offers nothing that would need resolution
```
