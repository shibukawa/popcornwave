---
title: Security Headers
description: The browser policy headers every response carries, the two you have to write yourself, and why HSTS waits for a verified HTTPS request.
sidebar:
  order: 8
---

Three security headers are on every response before you configure anything:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
```

These are the ones with a defensible default. Nosniff is right for every
application. Refusing to be framed is right until you know you want to be
framed. And `strict-origin-when-cross-origin` sends a full referrer within your
own site and only an origin outside it, which is what most applications would
have picked.

The two headers that actually shape a page — Content-Security-Policy and
Permissions-Policy — are empty, because there is no default for them that is
both safe and useful. A policy strict enough to be worth having is a policy
written against the assets a specific page loads.

## The keys

```toml
[security.headers]
enabled = true
content_type_options = true
frame_options = "deny"
referrer_policy = "strict-origin-when-cross-origin"
content_security_policy = ""
content_security_policy_report_only = ""
permissions_policy = ""
```

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
