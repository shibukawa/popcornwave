---
id: requirement:editor-workspace-symbols
type: requirement
title: Workspace Symbol Search Across Dialects
---
Every declaration in every .pw.* source of the project is findable by name from one search, so a component, a statement, and a page are reachable without knowing which dialect declared them.

```yaml
status: implemented at internal/pwlsp
stage: 2 of vision:editor-support
server: requirement:pw-language-server
why: requirement:pw-language-server lists documentSymbol and stops there, so finding a declaration requires already knowing its file
index:
  built_from: the same project model the server loads, so no second scan of the tree
  contains: component, statement, dynamo statement, page, layout, and external declarations
  excludes: a *_pw_gen.go symbol, because policy:generated-artifacts makes it output and gopls already indexes it
  scope: the decision:explicit-generation-sources purposes, so a .pw.html outside every declared purpose is absent rather than silently included
  freshness: updated from open-document changes without saving, and from a file-system change for closed files
presentation:
  kind: mapped to an LSP symbol kind that distinguishes the dialects, so a result list is readable without opening anything
  container: the declaring file, and for a page the route it serves, per concept:page-tree
  match: name substring and camel-hump, which is what the editor's own symbol search users expect
acceptance:
  - a component declared in one file is found from a document in another
  - a page is found by its route as well as by its declaration name
  - no result names a *_pw_gen.go
  - a declaration added to an unsaved document is findable before the save
  - a project that has never generated still returns every declaration
```
