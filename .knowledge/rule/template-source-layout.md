---
id: rule:template-source-layout
type: rule
title: Template Source Layout
---
Every .pw.html source the framework writes indents the body of a brace-delimited block one level, so a template reads as the nesting it compiles to.

```yaml
applies_to:
  - the api:cli-init scaffold, including ui:starter-landing-page, the requirement:nested-html-templates document shell, layouts, page-tree pages, and the error templates
  - the templates api:cli-add and api:cli-new write
  - the template examples in documentation
excludes: policy:generated-artifacts output, which gofmt owns
enforced_by: nothing; the api:cli-init .editorconfig carries the indent width so an editor does not undo it, and no command checks it
layout:
  block_body: indented one level from the line that opens the block
  component_body: no exception; the component brace is a block like any other
  closing: the closing brace or closing tag sits at the level of the line that opened it
  control_blocks: an if, else, or for block indents its body the same way, which is what they already did
defect_this_fixes:
  before: the component body sat at column zero while the control blocks inside it were indented, so the outermost block was the only one whose nesting was invisible
  effect: the markup read as if it were beside the component rather than inside it
non_goals:
  - a formatter command; this is how sources are written, not a check
  - a rule over application-owned templates after the scaffold has written them
```
