---
id: decision:preference-signal-precedence
type: decision
title: Preference Signal Precedence
---
CSS answers the unoverridden preference, a cookie carries the explicit override, and a client hint is an optional accelerator that no path depends on.

```yaml
status: proposed
source: user question 2026-08-10, asking whether Accept-CH should drive rendering
question: which signal decides the preference a server-rendered document is built with
precedence:
  order: evaluated as written, first present wins
  steps:
    - id: override
      signal: the preference cookie written by an application api:server-action
      verdict: use it
      why: it is the only signal that carries a choice the reader made on this site, and it is the only one CSS cannot see
    - id: hint
      signal: Sec-CH-Prefers-Color-Scheme and the other user-preference hints of data:preference-hint-config
      verdict: use it
      why: it reports the operating-system preference at request time, which lets a server-chosen attribute match the CSS result instead of contradicting it for one frame
    - id: absent
      verdict: report absent, per policy:absent-rather-than-stubbed
      why: substituting light here is a guess that repaints for exactly the readers this requirement exists for; the application answers it in CSS instead
css_is_the_floor:
  fact: prefers-color-scheme in CSS gives a correct first paint with no server knowledge, no Vary, no extra round trip, in every shipping browser
  consequence: the hint buys nothing for the unoverridden case, and a design that needs the hint to be correct is broken for Firefox and Safari readers
  what_the_hint_actually_buys: a server-emitted attribute or asset choice, such as the data-theme of requirement:daisyui-integration, that CSS alone cannot set on the document element
  rule: an application must remain correct with every hint absent, so the hint is an optimization and never a dependency
browser_reach:
  measured: MDN browser-compat-data, read 2026-08-10
  chromium: Sec-CH-Prefers-Color-Scheme from Chrome 93, Sec-CH-Prefers-Reduced-Motion from 108, Sec-CH-Prefers-Reduced-Transparency from 119, Sec-CH-Viewport-Width from 97, Critical-CH from 91
  firefox: none of them
  safari: none of them, desktop or iOS
  status: every one is flagged experimental; Critical-CH is not on the standards track
  conclusion: the non-hint path is not a fallback, it is the majority path for a large share of readers and the only path for some, which is why the cookie ranks above the hint and CSS sits under both
cookie_beats_hint_for_the_override:
  reach: every browser, against Chromium only
  authority: it holds a choice the reader made, which no hint reports
  control: the server writes it, so it survives, rotates, and clears on the server's terms under policy:cookie-value-protection
  cost: one Vary on the preference cookie, scoped by policy:preference-vary-correctness to responses that actually read it
first_navigation_gap:
  fact: a client sends no hint until an Accept-CH response has taught it, so the first navigation to an origin never carries one
  options:
    accept_the_gap: the first page renders from the cookie or from CSS, later pages may use the hint; costs nothing
    critical_ch: the client discards the response and retries once with the hints attached, before rendering
  chosen: accept the gap by default, with Critical-CH available per origin through data:preference-hint-config
  why: decision:scriptless-browser-detection rejected inverted_cookie because the cost landed on every client's first view, and that first view is the cold visit worth the most; Critical-CH puts a full round trip exactly there, for a signal CSS already answers
  when_critical_ch_earns_it: an operator whose document element must carry a server-chosen theme attribute, who has measured the flash, and who accepts one extra round trip per origin on cold navigation
viewport_width:
  reading: permitted, for asset and srcset sizing under policy:asset-transform-matrix and requirement:derived-asset-pipeline
  branching: refused; a handler must not select markup structure by width
  why_refused:
    staleness: width changes on rotation and resize with no new navigation, so the decision is wrong while the reader is still reading
    cache: a near-continuous integer makes a shared cache miss on nearly every request, per policy:preference-vary-correctness
    the_css_answer_is_better: container queries and srcset decide per element, continuously, in every browser
  never_varied_on: policy:preference-vary-correctness states the rule this points at
rejected_alternatives:
  hint_as_the_primary_signal:
    shape: Accept-CH plus Critical-CH, with the server treating the hint as the source of truth
    why_not: correct for Chromium only, so every application still needs the cookie and CSS paths, and building all three makes the hint pure addition
    what_it_would_cost: a Vary on every preference-bearing page and a round trip on cold navigation, to replace a CSS feature that already works
  script_bootstrap:
    shape: an inline head script reading storage and setting the attribute before paint
    why_not: policy:security-response-headers keeps script-src to self with no nonce, and decision:runtime-tag-injection admits external references only, so no path here writes inline script
    also: it is the flash this requirement exists to remove, merely made shorter
  user_agent_inference:
    shape: guess a preference or a width from the User-Agent
    why_not: decision:bot-client-classification confines that header to render-branch selection, and a device name predicts neither a theme choice nor a window size
  framework_owned_theme:
    shape: the framework ships the theme names, the toggle, and the CSS
    why_not: requirement:daisyui-integration keeps component structure and theme selection application-owned, and this would reverse it
security:
  unauthenticated: every signal here is client-declared, so none may reach an access decision, matching the no_verification corollary of decision:bot-client-classification
  cookie_scope: the preference cookie carries a presentation value only, never identity, and is readable without a session under decision:lazy-cookie-session-loading
  fingerprinting: the hints are declared only when configured, and are requested from the origin rather than delegated to third parties
criteria:
  - an application with no cookie, no hint, and correct CSS renders correctly in every browser and varies on nothing
  - an application that adds the cookie renders the override correctly in every browser
  - an application that adds the hint improves Chromium cold navigation and changes nothing elsewhere
  - removing hint support from a working application leaves it correct and slower, never broken
  - no code path reads a preference to decide routing, authorization, rate limiting, or text content
```
