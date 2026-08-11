---
id: api:user-preference-accessors
type: api
title: User Preference Accessors
---
A handler reads a resolved presentation preference through one accessor per preference, each reporting absence explicitly and each recording the Vary its read implies.

```yaml
surface:
  - pw.ColorScheme(context.Context) (pw.ColorSchemePref, bool)
  - pw.ReducedMotion(context.Context) (bool, bool)
  - pw.ViewportWidth(context.Context) (int, bool)
placement: pw, beside the other accessors of api:request-context-accessors
shape: the second return reports presence, so an application distinguishes absent from a preference that happens to equal the default
shape_direction: policy:request-scoped-accessor-shape renames these with a Context suffix and adds a request-handle form when it is applied to the accessor surface as a whole
values:
  ColorSchemePref: light or dark
  ReducedMotion: true when the client asked for reduced motion
  ViewportWidth: CSS pixels, ceiling-rounded by the client
resolution: decision:preference-signal-precedence, evaluated per accessor, so a cookie carrying only a color scheme leaves motion resolving from its own hint
absent_is_not_a_default:
  rule: no accessor substitutes light, false, or a width when nothing arrived
  why: policy:absent-rather-than-stubbed; a substituted value is indistinguishable from a real one and silently becomes the guess that repaints
  what_the_application_does: leave the attribute unset and let CSS answer it, which is the correct first paint anyway per decision:preference-signal-precedence
vary_is_a_side_effect_of_reading:
  fact: an accessor that resolved from a cookie or a hint records that signal on the request
  effect: api:html-response emits the Vary for exactly the signals that were read, per policy:preference-vary-correctness
  why_not_declared: a handler that must remember to declare a Vary forgets it, and the failure is a shared cache serving one reader's theme to another
  why_not_global: a page that reads nothing keeps an unvaried cacheable response, matching the scoping decision:bot-client-classification already made for User-Agent
  viewport_exception: pw.ViewportWidth records nothing, because policy:preference-vary-correctness forbids varying on it
not_exposed:
  to_templates: no generated parameter or template accessor carries a preference
  why: a template reaching it turns a presentation default into a content decision, which requirement:user-preference-rendering forbids for the reason decision:bot-client-classification gives
  how_it_reaches_markup: the handler resolves it and passes it as an ordinary typed parameter, so the value is visible in the page signature
no_middleware:
  choice: a pure function over the request and its cookie, evaluated where the answer is needed
  why: nothing must run before it, matching api:client-classification, so no route can miss it and no ordering requirement appears
  cost: repeated calls repeat the parse, which is bounded and allocation-free when the header and cookie are absent
configuration: data:preference-hint-config; preference_hints disabled leaves every accessor reporting absent, so an application keeps its CSS behavior with no code change
embedding: no configuration binding present reports absent rather than panicking, matching the accessor rules of api:request-context-accessors
testing:
  absent: no cookie and no header, which is the default in every test
  hint: a Sec-CH-Prefers-Color-Scheme header, which needs no Accept-CH round trip in a test
  override: the preference cookie, which outranks a contradicting header and is the case worth asserting
  determinism: no clock, network, or shared state
```
