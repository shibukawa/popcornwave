---
id: requirement:declaration-rename
type: requirement
title: Declaration Rename as a Generator Operation
---
Renaming a .pw.* declaration is performed by the generator's own view of the project and offered to the editor through it, because the name decides a generated Go symbol, its call sites, and sometimes a route.

```yaml
status: implemented. pw rename previews the edit set and --apply writes it; requirement:pw-language-server serves textDocument/rename from the same plan
route_note: >
  no route moves. The route among what a name decides is the concept:page-tree
  directory name rather than the declaration inside it, so renaming the
  declaration in a page.pw.html leaves the URL where it was. Renaming the
  directory is a different operation and this is not it, which is why the
  separate confirmation below has nothing to confirm
stage: 4 of vision:editor-support
why_not_an_editor_refactor: requirement:pw-language-server defers rename for this reason; this requirement is where the deferral is answered rather than reversed
what_a_name_decides:
  generated_symbol: the exported Go function flow:template-generation emits
  call_sites: handwritten Go calling that function, which the editor cannot rewrite from a template edit alone
  template_references: a component reference in another .pw.html body
  route: a concept:page-tree directory name, where renaming the directory is a route change and rule:page-directory-naming constrains the result
  fixture: a requirement:template-storybook fixture named for the template
surface:
  cli: the operation runs in system:pw-cli, so it works with no editor and is testable without one
  editor: requirement:pw-language-server exposes textDocument/rename by delegating to it, so the editor gains no second implementation
  preview: the edit set is returned before it is applied, because it crosses files the developer did not open
scope_rules:
  writes: source files only; a *_pw_gen.go is regenerated rather than edited
  refuses: a rename that would collide with an existing declaration, an unexported-to-exported change that alters what requirement:template-storybook can reach, or a page directory rename rule:page-directory-naming rejects
  route_change: reported as a route change and confirmed separately, because renaming a page directory changes a URL a user may have bookmarked
  out_of_project: refused; a rename needs the project model requirement:pw-language-server loads
non_goals:
  - renaming a Go symbol, which gopls owns
  - renaming across a concept:component-package boundary, where the consumer is another repository
  - a rename that regenerates as part of the edit; regeneration stays the explicit action policy:editor-tool-execution requires
acceptance:
  - renaming a component updates its declaration, every template reference, and every Go call site of the generated function in one edit set
  - renaming a statement whose generated function is called from Go reports the Go call sites in the preview before anything is written
  - renaming a page directory reports the route that changes and is confirmed separately
  - a rename colliding with an existing declaration is refused with both positions named
  - no *_pw_gen.go is written by the rename itself
```
