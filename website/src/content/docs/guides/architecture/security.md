---
title: Security
description: What the framework defends by default, what it hands you, and where a request is checked before your handler runs.
sidebar:
  order: 4
---

Most of a framework's security is decided before you write a handler. The
question is which defenses are on when you have written nothing, which ones
wait for a configuration line, and which ones you own outright — because a
defense you assumed was on is worse than one you know is off.

This page follows one request through the checks, then covers what sits outside
that path.

## What a request passes through

A request reaches your handler through a chain, and the order is not arbitrary.
Each stage depends on the one above it having already run.

| Stage | What it decides |
| --- | --- |
| Operational endpoints | Health, readiness, and framework assets answer above everything |
| Public assets | A static file is served without touching a route |
| Body limit | An oversized body is refused before it is read |
| Security headers | The response policy the browser will apply |
| Recovery | A panic becomes a response instead of a dropped connection |
| **Session** | The cookie becomes a validated record, or the request is anonymous |
| **Authentication** | The login endpoints answer their own paths |
| **CSRF** | An unsafe request proves it came from your page |
| **Guard** | An unauthenticated request to a protected path is redirected |
| Your handler | — |

The three bold stages are where identity turns into authority. Everything above
them is about the shape of the request; everything from `Session` down is about
who is making it.

CSRF sits below authentication for a reason that is easy to miss. The login
endpoints answer above it, so a login POST and an OIDC callback never need a
token — they never reach the check at all. That is not an exclusion you
configure; it falls out of where the stage is.

## What is on by default

Three things protect a project that configured nothing.

**Response headers.** `X-Content-Type-Options`, `X-Frame-Options`, and a
referrer policy go out on every response. See [Security
Headers](/guides/frontend/security-headers/) for the two you have to write
yourself.

**Escaping.** A template inserts values through typed holes, and each one is
escaped for the position it lands in. Producing raw markup takes an explicit
intrinsic, so it is visible in review rather than implicit in a helper.

**Opaque session tokens.** The browser holds 256 random bits. The store is keyed
by a SHA-256 hash of that value, so a leaked database dump cannot be replayed as
a cookie.

## What waits for a line of configuration

**CSRF** is off until you name the paths it covers. That is deliberate: a check
installed over nothing reads as protection that is not there, and `csrf.include
= []` would be a worse lie than `csrf.enabled = false`.

`pw init` writes the shape and leaves it off:

```toml
[security]
csrf.enabled = false
csrf.include = ["/**"]
csrf.exclude = []
```

**Path protection** is the same shape. `protection.include = []` means every
path is public, and a project opts routes in rather than out.

**HSTS** is off until a deployment is certain about its certificate, because
turning it on is hard to undo in a browser that already cached it.

## How CSRF works here

A CSRF token proves a request came from a page your server rendered. The
mechanism is a secret the server knows and the attacker's page cannot read.

### The secret

One secret per session, minted when the session is created and stored in the
session record. Rotation is automatic: logging in replaces the session, and the
secret goes with it, so a token minted before a login is refused after one.

The secret never reaches a handler. It is deliberately absent from the session
view your code reads, and it never enters template scope.

### The three channels

The token reaches the browser three ways, and all three verify against the one
secret.

| Channel | Written by | Read by |
| --- | --- | --- |
| Hidden form field | The template compiler, into every unsafe form | A plain form submission |
| `pw_csrf` cookie | The session manager, beside the session cookie | The browser runtime |
| `X-CSRF-Token` header | The runtime, from the cookie | The server |

The hidden field is the part you never write. Any `<form method="post">` — or
put, patch, delete — gets one as its first child at generation time. A GET form
does not, because its fields become the query string and a token in a URL
reaches history, logs, and referrers.

Two forms of the same page are refused at generation rather than at runtime: a
form posting cross-origin, which would hand your session's secret to a third
party, and a form whose method is computed, which cannot be classified as safe
or unsafe.

### Why the cookie exists

The header could have come from the page. It does not, and the reason is
rotation.

A token embedded in the rendered HTML is fixed at render. If the session rotates
while the page is open — a login in another tab, a privilege change — the page
holds a token the server no longer accepts, and the next action fails with
nothing on screen explaining why. Reading the cookie at the moment a request is
issued closes that: the rotation wrote a new cookie, and the runtime picks it up
on its next request.

This is the shape Django, Laravel, and Spring's SPA configuration all use. It
costs one cookie that JavaScript can read, which is the one documented exception
to the rule that framework cookies are `HttpOnly`. A runtime that cannot read
the token cannot send it.

### Why the bytes differ every time

Each emission carries a fresh random pad, so two renders of one page produce
different token bytes even though both unmask to the same secret.

The reason is compression. Responses are compressed, and a secret that appears
unchanged in a compressed body alongside attacker-influenced text — an echoed
search query, a redisplayed form value — can be extracted a byte at a time by
watching response sizes. A pad that changes per response leaves nothing to
accumulate.

Verification rebuilds the expected value from the pad the request carried, then
compares in constant time. Rails and Django mask for the same reason.

### Anonymous visitors

A token needs a secret, and a secret normally comes from a session. A public
page with its own unsafe form — a contact form, a search POST — has neither.

Issuing a session for every anonymous visitor would work and is the wrong
trade: any unauthenticated request to that page writes a row, so a crawler
decides how many rows the store holds, and those rows expire rather than being
deleted, because there is no logout.

So the secret is the cookie, signed:

```toml
csrf.anonymous.enabled = true
csrf.anonymous.secret = "${SECURITY_CSRF_ANONYMOUS_SECRET}"
```

Verification recomputes the signature instead of reading anything, so the
anonymous population costs nothing to remember. The signature is what separates
this from the naive double-submit pattern, where anyone able to set a cookie
satisfies the check.

A session secret always wins, so logging in does not leave the anonymous one in
play.

### What a refusal looks like

A refused request gets 403 through the same error path as any other — your HTML
error page in a browser, a problem document for an API client — and your handler
is never called.

The response never says which check failed. The reason reaches the log, because
naming it in the response tells a caller which half to work on.

## What you still own

The framework does not decide these.

**Authorization.** A validated session says who is asking, not what they may
do. Path protection covers whole routes; anything finer is your handler's.

**Which paths are unsafe.** `csrf.include` is yours to narrow. So is
`csrf.exclude`, and a webhook belongs there: it has no session and carries its
own authentication.

**Registered component inputs.** Publishing a component as redrawable makes its
parameters attacker-controlled. A component that formats values handed to it is
safe; one that loads a record by identifier must check ownership itself, exactly
as a handler would. Registration is the review point.

**Secrets in configuration.** Values marked secret are redacted from logs and
the startup summary, and `pw doctor` reports a literal secret in a committed
file. It cannot tell you the value was already leaked.

## Checking what is on

`pw doctor` reads the merged configuration for the environment you name and
reports what is off:

```bash
pw doctor --env prod
```

It flags a disabled CSRF check, a session cookie without `secure`, weakened
response headers, and query diagnostics left on outside development. Running it
against production configuration from a developer machine is the point — the
findings are about the file, not the running process.
