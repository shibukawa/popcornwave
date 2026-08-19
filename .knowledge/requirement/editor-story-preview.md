---
id: requirement:editor-story-preview
type: requirement
title: Story Preview From the Editor
---
The component under the cursor is previewable while editing it, by reaching the requirement:template-storybook pane the running api:cli-dev already serves rather than by rendering anything in the extension.

```yaml
status: implemented; the server answers pw/storyFor with the URL built from the declared console port, and the extension opens it
stage: 3 of vision:editor-support
subject: the declaration under the cursor, resolved to its requirement:template-storybook story URL
why_not_a_renderer_in_the_extension:
  - requirement:template-storybook renders from the generated code and the resolved type graph, which only the running application has
  - a second renderer would be the reimplementation vision:editor-support rules out in its first principle
  - the story pane already answers the document-shell toggle, the emitted HTML, and the parameter set, none of which the extension would reproduce
tension:
  rule: requirement:editor-tasks states that the extension embeds no viewer and surfaces the api:cli-dev url as a link
  resolution: the same rule holds here; the extension computes which story URL corresponds to the cursor and opens it, in the editor's own simple browser when the developer asks for it in-window and in an external browser otherwise
  what_is_new: the mapping from a cursor position to a story URL, which is the part a link alone cannot do
behavior:
  requires: a running api:cli-dev, per policy:editor-tool-execution, which never starts it implicitly
  not_running: report that the loop is not running and offer the requirement:editor-tasks dev command; never start it from a preview
  follow_cursor: optional, so moving between declarations retargets the pane without a second command
  stale: after an edit the pane shows what the last generation produced, labeled, because requirement:template-storybook renders generated code
  unpreviewable: a declaration requirement:template-storybook has no story for reports that rather than opening an empty pane
non_goals:
  - rendering a template in the extension host
  - a second parameter-editing surface; the story pane owns parameters
  - previewing a page as a route, which requirement:editor-route-explorer links to instead
acceptance:
  - the command on a component declaration opens that component's story and no other
  - the command with no dev loop running reports why and starts nothing
  - an unexported component still previews, because requirement:template-storybook registers it from inside its own package
  - the pane reports staleness after an edit rather than showing new source with old output
```
