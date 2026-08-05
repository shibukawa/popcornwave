---
id: requirement:template-storybook
type: requirement
title: Template Storybook
---
A requirement:dev-console pane renders any generated template on its own, with supplied or synthesized parameters, so markup is reviewed without routing a request that reaches it.

```yaml
audience: actor:application-developer
pane_of: requirement:dev-console
mechanism: decision:dev-harness-process, which fits because a story renders from supplied values and reaches no database
subjects:
  - every .pw.html under the data:project-config generate.templates purposes
  - every concept:page-tree page template under the generate.pages purposes
  - the requirement:nested-html-templates document shell, as the wrapper a story may be rendered inside
  - the api:error-renderer templates of flow:error-template-generation, which have no ordinary route at all
visibility:
  problem: a generated Fragment named for an unexported template is unreachable from any package that is not its own
  resolution: api:cli-generate emits a pwdev-constrained registration file into the template's own package, so an unexported fragment registers itself from inside
  effect: the pane's subject list is what the project generated, not the subset the project chose to export
  bound: the registration file carries the build constraint of policy:dev-console-boundary, so api:cli-build emits and links none of it
parameters:
  fixture:
    location: beside the template, named for it
    content: named stories, each one parameter set
    absent: not an error; the synthesized set is used
    status: not delivered; every story renders from the synthesized set today, and a template needing particular values is the case that will ask for this
  synthesized:
    derived_from: the resolved type graph flow:template-generation already builds
    values: representative rather than zero, so a string field shows as text and a slice shows more than one element
    determinism: identical inputs produce identical values, because a story that changes every render reports nothing
  slots: filled with marked placeholder content, so slot placement is visible without inventing a child template
async:
  status: not delivered; an async parameter renders at whatever its zero value produces until this lands
  states: a story renders at the fallback state and at the resolved state, selectable
  reason: requirement:async-html-rendering makes the fallback a real rendering an application ships, and a route only shows it while it is racing
  bounds: policy:async-render-bounds unchanged; the pane resolves rather than waits
preview:
  form: the story is framed from a page carrying nothing of the storybook, so the harness stylesheet never reaches the markup under review
  raw: that page is also the plain output, which is what makes the frame possible without a second rendering path
shows:
  - the rendered result
  - the same result inside the document shell, toggled, because requirement:tailwind-css-integration styles reach it only there
  - the emitted HTML, so an escaping context from flow:template-generation is inspectable
  - the parameter set that produced it
lifetime: rebuilt with the same watched changes api:cli-dev already rebuilds on, so an edited template is re-rendered without a second command
non_goals:
  - a component catalog, a design-token viewer, or visual regression capture
  - editing a template or a fixture from the browser
  - interaction inside a story; concept:interaction-cost-ladder tiers belong to a running application
  - rendering a template the project did not generate
acceptance:
  - a template with no fixture renders from synthesized parameters
  - an unexported template appears in the pane
  - the same story rendered twice produces identical HTML
  - an await block renders once as its fallback and once resolved
  - a template that fails to render reports the diagnostic in place of the story and leaves the pane usable
  - a binary produced by api:cli-build contains no registration file and no story fixture
```
