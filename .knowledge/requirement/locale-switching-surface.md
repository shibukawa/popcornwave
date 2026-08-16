---
id: requirement:locale-switching-surface
type: requirement
title: Locale Switching Surface
---
One accessor answers what languages this page is available in, and the switcher control, the alternate links, and the canonical all read it.

```yaml
status: proposed
data:
  per_locale:
    - the locale tag
    - a label written in that language itself
    - the URL of this same page in that locale, under decision:locale-url-modes path mode only
    - whether it is the current one
  surface: api:locale-accessors LocaleChoices
one_source_three_consumers:
  switcher: the control an application renders, as anchors or a select
  alternates: the hreflang link elements, emitted through the head channel of decision:runtime-tag-injection
  canonical: the self-referencing link, and the x-default target
  why_it_matters: a page whose switcher and hreflang are computed separately drifts, and the drift is invisible until a crawler reports it
by_mode:
  path: every entry carries a URL, which is this path with its prefix replaced
  cookie: no URL exists, so switching is an api:server-action writing the cookie and redirecting back
  header: nothing is produced, because the client decides and no server control would be honest
labels_are_declared:
  where: data:i18n-config
  why_not_generated_from_CLDR: shipping display-name tables repeats the dictionary weight decision:message-id-assignment refused, for a value an application often wants to word itself
  why_not_a_message: the label is written in its own language regardless of the current one, so per-locale catalogs would hold the same value N times over
  missing: reported at generation, because falling back to the raw tag puts en in front of a reader instead of English
framework_and_application_split:
  framework: the data, and SetLocale writing the cookie through api:cookie-jar with the value validated against the declared set
  application: the markup, the control, and the action handler
  precedent: requirement:user-preference-rendering divides an override the same way, keeping the control application-owned
criteria:
  - a page in path mode lists one URL per declared locale, each resolving to the same page
  - the alternate links and the switcher never disagree, because one call produced both
  - a header mode route produces no switcher data and emits no alternate links
  - a locale with no declared label fails generation rather than rendering its tag
  - switching in cookie mode leaves the reader on the page they were reading
```
