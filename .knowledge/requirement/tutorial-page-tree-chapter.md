---
id: requirement:tutorial-page-tree-chapter
type: requirement
title: Tutorial Page Tree Chapter
---
A fifth tutorial chapter moves one route of the memo application into a concept:page-tree and uses it to introduce the three update models that make a Popcorn Wave page different from a template rendered once.

```yaml
audience: actor:application-developer
position: chapter 5, after the login chapter
continuity: requirement:tutorial-continuity, unchanged; the reader still types along and every block edits one they already have
capability: requirement:tutorial-capability-growth, whose chapter opens with pw add discovered
rendered_result: ui:tutorial-memo-app, extended with the page-tree route
why_here:
  - the four chapters before it build an ordinary server-rendered application, which is the part a reader can already picture
  - what they never show is why the framework has its own template language, and the three update models are the answer
  - the page tree is where two of the three cost the application nothing, so the chapter is one change with three payoffs rather than three features in a row
what_it_builds:
  route: one memo route under the page tree, beside the handlers the earlier chapters wrote
  coexistence: decision:dual-router-coexistence, so nothing built in chapters 1 to 4 is rewritten or deleted
  reader_sees: a directory holding page.pw.html serves a route with no registration, per requirement:discovered-page-routing
  layout: an ancestor layout, since a layout chain is also what the partial update section needs to be true
  rung:
    takes: the handler rung, func Load(w, r)
    why: the page reads the signed-in account and the database pool, both of which arrive on the request context, which requirement:page-request-context records the typed rung cannot reach
    says_so: the chapter explains that choice where it makes it, rather than letting the handler rung read as the ordinary shape a page takes
update_models:
  depth: an overview each, with the guide page carrying the rest
  order: partial, async, live
  ordering_reason: partial is what the reader already has by the end of the route section, async is one template keyword away, and live is the one that needs a connection and a source that does not end
  partial:
    owner: requirement:navigation-delta-rendering
    shown_as: the page the reader just built, already answering a same-tree navigation with the boundaries that changed
    application_delta: none, which is the point
    demonstrated_by: something the reader can observe, since a change with no code to show needs evidence
  async:
    owner: requirement:async-html-rendering
    shown_as: one slow value on the memo page arriving after the rest of it
    application_delta: an async parameter and an await clause with a fallback
    honest_about: the fallback is required, because a boundary has to render something before the value exists
  live:
    owner: requirement:live-html-rendering
    shown_as: one region of the memo page updating after the document is complete
    application_delta: an external live declaration, a Go source that yields many values, and the framework script the scaffold already writes
    honest_about:
      - a delivery replaces a whole boundary subtree, so a long list costs its length every tick
      - this is the model with a connection open, which is a deployment consideration the earlier chapters never raised
  guides:
    - discovered routing
    - partial updates
    - async rendering
    - live rendering
  linking_rule: the chapter links each guide once, where that model is introduced, rather than collecting them at the end
scope_discipline:
  rule: the chapter introduces the models; it does not tour their options
  excluded: validators and cache keys of requirement:navigation-delta-rendering, the bounds of policy:async-render-bounds, the recovery behavior of requirement:live-connection-recovery, and api:page-action-endpoint
  reason: a reader five chapters in wants to know these exist and what they cost, and the guides are where a reader who needs one goes next
  no_unbuilt_behavior: nothing the chapter shows may depend on the not_implemented items of requirement:live-html-rendering
knock_on:
  chapter_count: the getting-started chapter says the tutorial is four chapters, in both languages, and becomes five
  closing: the login chapter closes the tutorial today, and hands off to this one instead
acceptance:
  - a reader who typed every earlier chapter reaches a compiling project at the end of this one
  - the page-tree route serves without a registration the reader wrote
  - each of the three models is visible in the running application, not only described
  - no framework symbol appears before the chapter says what it is
  - the English and Japanese versions carry the same sections in the same order
non_goals:
  - converting the handler routes of chapters 1 to 4 to the page tree
  - a chapter per update model
  - teaching requirement:html-fragment-rendering or the authored-island tier of concept:interaction-cost-ladder
```
