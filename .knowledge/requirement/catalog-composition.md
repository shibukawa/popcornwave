---
id: requirement:catalog-composition
type: requirement
title: Catalog Composition
---
The framework, a component package, and the application each contribute messages, and generation flattens the three into one table before any of them is read.

```yaml
status: proposed
contributors:
  framework: the text of policy:validation-errors conversion failures, requirement:rate-limit-problem-responses, authentication failures, and the api:error-renderer pages
  packages: a module distributed under requirement:component-package-distribution, which already ships components and assets
  application: everything else
namespace:
  separation: one reserved prefix for framework messages and the declaring module's name for a package, above the scopes of decision:message-scope-declaration
  override: an application may supply its own translation for a contributed ID
  order: application over package over framework, resolved at generation
flattened_at_generation:
  fact: composition and the fallback chain are both static, so decision:message-code-shape fills one table
  effect: no runtime search across catalogs, and a composed catalog costs exactly what a single one costs
  reporting: an override that matches no contributed ID is reported, because it is a typo that would otherwise do nothing
framework_locale_coverage:
  bound: the framework ships the locales it can keep correct, not every declared locale
  gap: a declared locale the framework does not ship falls back per the chain, or the application supplies it
  rule: a framework gap is a warning and never a build failure, because an application cannot fix the framework's catalog
server_side_localization_is_opt_in:
  default: api:problem-response keeps returning a machine-readable code and unlocalized text
  why: a native client knows its device locale exactly, works offline, and ships translations matched to its own version, so a server-chosen string is usually worse
  already_available: the problem body already carries a code, so the machine-readable path costs nothing new
  when_the_server_must: mail, push notifications, generated documents, operator-configured text, and any client that cannot hold a catalog
  mechanism: the caller passes a rendered string into the pw constructors of api:problem-response, so the framework never translates on its own initiative
  html_pages: the api:error-renderer templates reference messages normally, since they are templates
dynamic_lookup:
  refused: resolving a message from a runtime string key, which is the untyped map lookup the framework removes elsewhere
  application_answer: a switch over a declared enum, written by hand
  future_option: a catalog declaring that a scope is keyed by an enum, letting generation emit the switch with exhaustiveness checked; not in the first release
criteria:
  - an application that overrides one framework message keeps every other contributed message
  - a package added to a project contributes its messages with no configuration beyond declaring the package
  - removing every contribution but the application's leaves the generated output identical to a project that never had them
  - a locale the framework does not ship produces a warning and a working build
```
