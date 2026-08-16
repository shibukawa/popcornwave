---
id: data:i18n-config
type: data
title: Internationalization Configuration
---
Almost every internationalization key changes generated output, so the block lives in data:project-config rather than in a runtime binding.

```yaml
location: the i18n block of data:project-config
keys:
  i18n.locales: declared locale tags, deciding which segment table columns and which plural rules are generated
  i18n.default_locale: the terminal of the fallback chain decision:message-code-shape flattens
  i18n.catalog: the catalog directory, addressed directly rather than discovered
  i18n.missing: severity for a locale missing a message, error or warn
  i18n.prefix_default: whether the default locale carries a path prefix
  i18n.path_routes: path prefixes whose locale is a URL segment
  i18n.cookie_routes: path prefixes reading the locale cookie, then Accept-Language
  i18n.header_routes: path prefixes reading Accept-Language only
  i18n.label.<tag>: one display label per locale, written in that locale
why_build_time:
  locales: a locale absent here emits no table column, so it cannot be enabled at run time
  prefix_default: it decides whether link-collapsing code is emitted at all, so a negotiated project generates none
  url_style_generally: an application does not move between URL shapes during operation, and treating it as operational would cost generated code in every project that never uses it
runtime_binding:
  members: the locale cookie name
  registration: its own binding under decision:independent-runtime-config-bindings
  why_separate: it is the one value an operator legitimately changes per deployment
route_entries:
  form: one array per mode, so the mode is the key name rather than a field of a table
  why_not_an_array_of_tables: three arrays read as well and need no array-of-tables support from the configuration parser, which minitoml would have had to grow for one block
  match: longest prefix, evaluated against the path after locale stripping
  unlisted_path: the default locale with no negotiation, so a project adding i18n to one subtree leaves the rest alone
  registered_into_the_binary: the generated message package emits the route list, the labels, and prefix_default, because a served process cannot read popcornwave.toml
rules:
  - a locales list that omits default_locale is a startup error
  - a locale tag that is not well-formed BCP 47 is a startup error
  - a locale with no label entry is reported at generation, per requirement:locale-switching-surface
  - a route entry naming an unknown mode is a startup error
  - two route entries with the same prefix is an error, since the later would never be reached
  - prefix_default false with one declared locale is accepted and generates no prefix handling
  - an absent i18n block means the project is single-locale, which is the shape every project written before this had
scaffolded: api:cli-init writes no block, so a new project has nothing to remove
migration: an existing project adds the block; nothing about it is inferred from the presence of a catalog directory
generation_purpose:
  chosen: none; i18n.catalog is a fixed path of decision:explicit-generation-sources rather than a purpose
  why: a purpose is a list a walk discovers sources under, and this is one directory addressed directly, so a second key naming it would be the same fact twice
  stale_sweep: the generated message package is exempt from the outside-every-purpose report for the reason the asset manifest is, being written deliberately by something other than a directory walk
  not_a_second_reader_of_templates: a reference inside a template is found by the runs that already compile it and validated against the generated symbols, so no purpose scans .pw.html twice
consumers:
  - api:cli-generate
  - api:locale-accessors
  - policy:locale-vary-correctness
  - rule:locale-prefix-checks
```
