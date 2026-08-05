---
title: Security Headers
description: The default browser policies, how to replace them safely, and why HSTS waits for a verified HTTPS request.
sidebar:
  order: 8
---

Three security headers are on every response before you configure anything:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
```

These defaults are broadly safe: `nosniff` prevents content-type guessing,
`DENY` blocks framing until the application explicitly allows it, and
`strict-origin-when-cross-origin` sends a full referrer only within the same
origin.

Content-Security-Policy ships with a default too, and it is deliberately narrow:

```
script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'
```

It restricts the four directives nearly every application can accept and leaves
alone the ones it cannot. Images, fonts, styles, and connections are
unrestricted, so a page that loads a logo from a CDN keeps working without
anyone editing configuration.

`script-src 'self'` is the one doing the work. It refuses inline event handlers,
inline `<script>`, and `javascript:` URLs — together, the ways an HTML-injection
sink turns into running code. It matters more here than in a framework without a
browser runtime, because the [CSRF](/guides/architecture/security/) companion
cookie is readable by script on purpose: script that runs on your origin can mint
a valid token. The framework's own runtime is a same-origin module tag and needs
nothing beyond `'self'`.

This is the second line rather than the first. Templates already refuse a
`javascript:` URL where it is written, before any header is consulted — see [URL
attributes](/guides/frontend/templates/#attributes). The header is what still
holds when markup reaches the page some other way.

If the application loads third-party scripts, replace the default with a policy
that names their origins explicitly.

Permissions-Policy is still empty, because a default for it would be a guess
about features your application may not use at all. If a reverse proxy already
owns either policy, keep one source of truth instead of setting another here.

## The keys

```toml
[security.headers]
enabled = true
content_type_options = true
frame_options = "deny"
referrer_policy = "strict-origin-when-cross-origin"
content_security_policy = "script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
content_security_policy_report_only = ""
permissions_policy = ""
```

Setting `content_security_policy` replaces the default outright rather than
adding to it. Setting it to `"off"` sends no policy at all — an empty value means
the default, so a project that genuinely wants no policy needs a way to say so
that is not silence.

`frame_options` takes `deny`, `sameorigin`, or `off`. `referrer_policy` takes
`no-referrer`, `same-origin`, `strict-origin`, or
`strict-origin-when-cross-origin`. Any other value fails startup rather than
reaching a browser that would ignore it — a misspelled policy is a policy you do
not have, and finding that out from a running site is finding out too late.

Every value is checked for control characters as well, so a header cannot be
split by something that came out of a config file.

Three keys have no environment-variable binding: `content_security_policy`, its
`_report_only` twin, and `permissions_policy`. Set them in TOML. They are long,
they contain characters that shell quoting mangles, and they belong in a file
you can read in a diff.

## Writing a policy

The report-only variant is how you find out what a policy would break before it
breaks anything:

```toml
[security.headers]
content_security_policy_report_only = "default-src 'self'; report-uri /csp-report"
```

Both keys can be set at once — the browser enforces one and reports on the
other, which is how a policy is tightened without a flag day.

If you serve the [API documentation UI](/productivity/api-documentation/), you
do not need to widen your policy for it. That endpoint replaces the policy on
its own response with the one it needs, per response, so the CDN host and inline
allowances it requires never reach any other route.

## HSTS

```toml
[security.headers.hsts]
enabled = true
max_age = "8760h"   # one year; durations have no day unit
include_subdomains = true
preload = false
```

`Strict-Transport-Security` is off by default, and even when enabled it is sent
only on a request that arrived over HTTPS. Announcing over plaintext that a site
is HTTPS-only asks a browser to remember a claim the connection could not vouch
for.

"Arrived over HTTPS" means a direct TLS connection. Behind a terminating proxy
there is none, so the framework reads `X-Forwarded-Proto` — but only from a peer
listed in `server.trusted_proxies`. Without that list the header is ignored,
because any client can send it.

```toml
[server]
trusted_proxies = ["10.0.0.0/8"]
```

Two guards on the values: `max_age` must be positive when HSTS is enabled, and
`preload` additionally requires `include_subdomains` with a `max_age` of at
least a year. Those are the browser preload list's own requirements, and
submitting a header that does not meet them wastes a submission.

Start with a short `max_age` and raise it. A `max_age` you regret is a `max_age`
every returning browser honours until it expires.

## Turning it off

`enabled = false` registers no middleware at all. It is there for the
application that sits behind a gateway already setting these headers, where two
layers setting `X-Frame-Options` differently is worse than one layer setting it.

For the [operational endpoints](/guides/deployment/operational-endpoints/) —
health and readiness — none of this matters much; they answer plain text and
reveal only status.
