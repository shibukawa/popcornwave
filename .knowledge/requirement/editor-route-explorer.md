---
id: requirement:editor-route-explorer
type: requirement
title: Route and Page Tree View
---
The routes a project serves are listed in one view, with the file that answers each one, because decision:dual-router-coexistence means no single directory listing shows them all.

```yaml
status: implemented. Discovered routes are walked from the sources, and the registered half is read out of data:route-table
registered_routes:
  read_not_analyzed: finding them needs the resolved import graph, so the view reads what api:cli-generate wrote rather than running that analysis a second time; api:cli-doctor reads the same table for the same reason
  freshness: the table is as fresh as the last generation, while the page tree is walked from the sources on every request, so a page added a moment ago is there and a registration added a moment ago is not
  absent_table: said in the view rather than shown as an empty list, because a view that silently covers one router reads as if it covered both
  unresolved: stated as what the view does not cover, per the data:route-table rule that a consumer names its limits rather than reporting a table it cannot back up
stage: 3 of vision:editor-support
audience: actor:application-developer
problem:
  - a discovered route lives in a concept:page-tree directory name, and a registered route lives in a flow:handler-registration call, so the URL space is spread across two representations
  - a developer answering what serves this URL reads both, and a developer answering what URL does this file serve reads neither easily
content:
  discovered: every concept:page-tree route, its page template, its layout chain, and its optional page.go
  registered: every literal route of flow:handler-registration, with the handler it names
  actions: the api:page-action-endpoint handlers a page declares, nested under it
  errors: the api:error-renderer templates, which serve no ordinary route and are otherwise invisible
  source: the project model requirement:pw-language-server loads, so the view and the diagnostics agree by construction
behavior:
  jump: selecting an entry opens the file that answers it, at the declaration rather than the top
  reverse: the view reveals the entry for the active editor, so the file answers what URL does this serve
  conflict: a route both routers could answer is marked, since decision:dual-router-coexistence makes that a real state rather than an error
  open_in_browser: a route offers the running api:cli-dev URL as a link, and nothing when the loop is not running
non_goals:
  - creating a route from the view, which would write files from a click; requirement:editor-code-actions owns file creation
  - rendering anything; requirement:editor-story-preview owns preview
  - a view that requires generated output, since routes are derivable from source alone
acceptance:
  - a page directory added on disk appears without a regeneration
  - selecting a route opens its page template
  - the entry for the active editor is revealed without a search
  - a project with only registered routes shows them and shows no empty page tree
```
