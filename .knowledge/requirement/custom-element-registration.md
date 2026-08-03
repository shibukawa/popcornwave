---
id: requirement:custom-element-registration
type: requirement
title: Custom Element Registration
---
system:tinybind v0.3.3 closed the hyphenated element namespace, so a template writing a Web Component must declare it and an undeclared one is a generation error; this framework has to expose that declaration to projects, because the authored-islands tier of concept:interaction-cost-ladder is built entirely out of custom elements.

```yaml
what_changed_upstream:
  before: a hyphenated element was emitted verbatim, unread
  now: every hyphenated element resolves against a registered whitelist, and an undeclared one fails generation naming the file, line, and column, with a suggestion for a near name
  disjoint_by_construction: a standard HTML element name carries no hyphen, so the whitelist can never shadow one
  excluded: the interiors of SVG and MathML
  two_kinds:
    passthrough: a name or a glob, emitted verbatim and producing no plan step; this is what an application's Web Components need
    builtin: markup plus a provider symbol, rewritten at generation time into plan steps; this is a framework capability rather than an application one
why_it_reaches_this_framework:
  authored_islands: concept:interaction-cost-ladder names custom elements as the boundary of its authored-islands tier, so every project that reached that tier writes hyphenated elements
  scaffolds_unaffected: no template this framework scaffolds writes one, so a project that never adopted that tier regenerates identically
  the_gap: a project that did adopt it now fails generation with no way to declare its elements, because api:cli-generate exposes no such option
surface:
  where: data:project-config, as a generation list, since a passthrough entry carries no Go symbol and needs no Go construction
  form: exact names and globs, so a component library costs one entry rather than one per element
  applied_by: api:cli-generate, alongside the template patterns and the generated-header prefix it already sets
  diagnostics: the generation error already names every element to declare, so the remedy is copyable into the configuration
  checks: rule:route-and-template-checks reports a declared element no template uses, since a stale whitelist quietly widens what compiles
builtin_elements:
  needed_today: none; requirement:module-native-csrf removed the only one this framework had a use for
  kept_open: the seam exists and is a framework-level option rather than a project one, so a future capability declares its element in Go and every project's templates may write it
  what_a_definition_carries: the markup, an optional provider symbol, head-or-body placement, the request properties its output varies on, and the assets it requires
  vary_reaches_the_response: a declared vary axis folds into the composition and is readable before rendering, which is what lets a response set an honest Vary header rather than guessing
migration:
  breaking_for: a project already using Web Components, which is precisely the tier this framework told authors to reach for
  cost: one configuration list, once, with the diagnostic naming its contents
  not_silent: generation fails rather than emitting something subtly different, so nothing reaches a browser broken
acceptance:
  - a project using no hyphenated element regenerates byte-identical output
  - a project using Web Components declares them once in configuration and generates
  - a misspelled element fails generation with a position and a suggestion
  - a component library is covered by one glob
  - api:cli-doctor or rule:route-and-template-checks reports a declared element nothing uses
  - a scaffolded project generates without any declaration, because it writes none
```
