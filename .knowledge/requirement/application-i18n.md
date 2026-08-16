---
id: requirement:application-i18n
type: requirement
title: Application Internationalization
---
An application serves text in the reader's language, and a missing translation is a build failure rather than a production discovery.

```yaml
status: proposed
axes:
  text: message catalogs reaching templates and Go, per decision:message-source-of-truth
  urls: decision:locale-url-modes
  assets: requirement:localized-assets
framework_owns:
  - the catalog format, its composition, and the generated typed accessors of decision:message-code-shape
  - locale resolution per route, per decision:locale-url-modes
  - the Vary each mode implies, per policy:locale-vary-correctness
  - build-time reporting of missing translations, placeholder mismatch, and missing plural categories
  - the data a switcher and hreflang need, per requirement:locale-switching-surface
application_owns:
  - the catalog contents
  - where the language appears in a URL, written explicitly per decision:explicit-locale-in-links
  - the switcher markup and the api:server-action that records a choice
  - which assets are localized
separate_axis_from_preferences:
  fact: requirement:user-preference-rendering forbids a preference deciding text content, and decision:preference-signal-precedence names text content in the same exclusion
  consequence: language never rides the preference cookie or the accessors of api:user-preference-accessors; it carries its own signals and its own Vary rule
  why: a presentation default has a CSS floor that leaves an unread response correct for every reader, and language has no floor
  where_the_difference_is_recorded: policy:locale-vary-correctness, which states the failure the shared rule would produce
single_language_cost:
  fact: a project that never declares a second locale writes ordinary text in templates and never reaches for decision:message-scope-declaration
  migration: an existing project marks text and runs the extraction of decision:message-id-assignment, which is mechanical
non_goals:
  - a framework-owned theme, layout, or component variant per locale; only text, URLs, and assets differ
  - locale as an input to routing beyond the prefix, authorization, or rate limiting
  - runtime lookup of a message by a string key, which decision:message-code-shape replaces with a typed call
  - bundling CLDR display names or a morphological dictionary, per decision:message-id-assignment and requirement:locale-switching-surface
  - server-side translation of api:problem-response bodies by default, per requirement:catalog-composition
criteria:
  - a build with a locale missing one message fails, naming the message and the locale
  - a translation whose placeholder set differs from its declaration fails the build in every configuration
  - a reference passing a wrong argument count or a misnamed argument fails generation at the argument; a wrong argument type fails as a Go compile error instead, per decision:message-code-shape
  - a locale whose plural rules require a category the catalog omits fails the build
  - adding a locale changes generated data and adds no generated code, per decision:message-code-shape
  - a project declaring one locale carries no runtime fallback branch, because generation flattened it
  - no response varies on a signal the route's mode does not read, and every response in a negotiated mode varies whether or not this request carried the signal
```
