---
id: decision:locale-url-modes
type: decision
title: Locale URL Modes
---
A route declares one of three ways its locale is decided, and that declaration is also what its Vary is derived from.

```yaml
status: proposed
source: user discussion 2026-08-16
modes:
  path:
    decided_by: a URL path prefix
    for: public content that is indexed and reaches a shared cache
    vary: none, because the URL already distinguishes the representations
    alternates: hreflang and canonical are expressible, per requirement:locale-switching-surface
  cookie:
    decided_by: the locale cookie, then Accept-Language, then the default
    for: an authenticated application or an operator console, where the reader chose a language in a settings surface
    vary: the cookie and Accept-Language, unconditionally, per policy:locale-vary-correctness
  header:
    decided_by: Accept-Language, then the default
    for: an api answering a native client, which sends what its device reports and manages no cookie
    vary: Accept-Language, unconditionally
    no_switcher: the client decides, so requirement:locale-switching-surface produces nothing here
why_cookie_and_header_are_not_one_mode:
  distinction: whether a stored reader choice outranks the request header
  consequence: an api whose body changes with cookie state is answering the same request two ways for reasons the client did not state
cookie_precedence_within_cookie_mode:
  order: cookie, then header, then default
  authority: decision:preference-signal-precedence gives the same reason for the same ordering, since a cookie holds a choice the reader made and a header reports an environment
  cookie_mode_only: the plain mode of policy:cookie-value-protection through api:cookie-jar, carrying a declared locale tag and no identity
  separate_from_the_preference_cookie: requirement:user-preference-rendering forbids that cookie deciding text content, so riding it would cross a line this project drew
per_route:
  form: prefix entries in data:i18n-config, longest match wins
  matched_against: the path after prefix stripping, which makes every case unambiguous
  cases:
    - "/ja/about strips to /about, matches a path route, prefix present, served"
    - "/about matches a path route with no prefix, redirected to the negotiated locale"
    - "/ja/admin/ strips to /admin/, matches a cookie route, prefix present, 404"
    - "/admin/ matches a cookie route with no prefix, served"
  second_matcher:
    fact: decision:stdlib-servemux owns routing, and this adds a prefix match beside it
    why_tolerable: it selects a policy scope rather than a handler, which is how authentication and caching scope are configured too
    future: the tree roots of decision:explicit-generation-sources are the natural unit if per-subtree declaration is ever needed
prefix_on_the_default_locale:
  setting: data:i18n-config prefix_default
  default: true
  why_the_default: changing which locale is default under prefix_default false moves every URL on the site, which is a migration; under true it changes only where the root redirects
  why_it_is_offered_at_all: decision:explicit-locale-in-links removed the cost that made supporting both expensive, and the empty-prefix path is already required by cookie and header modes
  when_false:
    - the root path is the default locale itself, so the negotiation redirect and its Vary disappear
    - the default locale's own prefix is redirected away permanently, so one URL serves one representation
root_path_under_prefix_default_true:
  behavior: negotiate and redirect
  status: 302 rather than 301, because the target depends on the request
  vary: Accept-Language on the redirect
  crawlers: send no Accept-Language, so they reach the x-default target
rejected_alternatives:
  subdomain:
    shape: a host per locale
    environment_cost: DNS records, certificates as a wildcard or an enumerated SAN list, the termination configuration of decision:ingress-tls-termination, and local name resolution for every developer under decision:local-tls-proxy-boundary
    cookie_cost: sharing a session across locales requires a Domain-scoped cookie, which gives up host-only cookies and the prefix that enforces them, so one subdomain takeover reaches the session; policy:cookie-value-protection protects the value and not the breadth of where it is sent
    origin_cost: policy:csrf-protection and decision:websocket-origin-check-owner move from one origin to a set, and a set is harder to verify
    why_path_wins: same origin throughout, so sessions, CSRF, and cookies never learn that locales exist
  accept_language_for_public_pages:
    why_not: policy:locale-vary-correctness states the cache outcome, which is why public content is the path mode's case
```
