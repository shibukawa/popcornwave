---
id: requirement:editor-generated-peek
type: requirement
title: Peek the Generated Go
---
The Go that a declaration produced is readable from the declaration, in a read-only view, so the framework's indirection is inspectable without opening an artifact policy:generated-artifacts calls output.

```yaml
status: implemented; the server answers pw/generatedFor and the extension publishes it under a read-only scheme
stage: 3 of vision:editor-support
server: requirement:pw-language-server
why:
  - requirement:editor-navigation resolves through a *_pw_gen.go and never lands in it, which is right for navigation and leaves no way to answer what did this generate
  - a developer learning the framework asks that question constantly, and the honest answer today is to open an uncommitted file and scroll
surface:
  view: a read-only virtual document holding the emitted Go for the declaration under the cursor
  scope: the declaration, not the whole file, because the whole file is what the developer already could open
  identity: a stable URI naming the source declaration, so the view refreshes rather than accumulating tabs
  editing: refused, with the reason named once; policy:generated-artifacts owns the file
content:
  present: the generated function, its signature, and the registration the declaration produced
  absent_output: the view reports that api:cli-generate has not run rather than generating on demand, per policy:editor-tool-execution
  stale_output: labeled as older than the source, using the same comparison api:cli-check makes
positions: requirement:template-source-positions, so a line in the view maps back to the template line that produced it
non_goals:
  - generating on open, which would write files from a keystroke
  - a diff against a previous generation, which is version control's job where the file is committed and meaningless where it is not
  - offering the view for handwritten Go
acceptance:
  - peek on a component declaration shows its generated renderer and nothing from the rest of the file
  - peek with no generated output present reports why and offers the requirement:editor-tasks generate command
  - an edit attempt in the view is refused once, not silently discarded
  - the view for a declaration whose source changed is labeled stale rather than shown as current
```
