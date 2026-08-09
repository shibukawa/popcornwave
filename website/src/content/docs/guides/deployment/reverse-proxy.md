---
title: Behind a Reverse Proxy
description: What a terminating proxy changes about HTTPS detection, the CSRF origin check, and progressive rendering, and which setting puts each one back.
sidebar:
  order: 4
---

Put nginx, an ingress controller, or a cloud load balancer in front of the
application and the process stops talking to a browser. It talks to the proxy.
`r.TLS` is nil on a request the visitor made over HTTPS, `r.RemoteAddr` is the
proxy's address rather than the client's, and everything the framework used to
know from the connection is now a header — which is to say, a claim made by
whoever sent the request.

The framework believes almost none of those claims by default. That is the
correct default for a process listening on a public port, and behind a proxy it
is wrong in three specific places: HSTS never appears, every form submission is
refused, and progressive rendering stops being progressive.

If nothing sits in front of your application — the process terminates TLS
itself, or it is reachable only from a socket you own — skip this page and leave
`server.trusted_proxies` empty. The empty list is what makes `X-Forwarded-Proto`
unspoofable, and naming a network you do not fully control gives that header
away to anyone who can reach the port from inside it.

The opposite arrangement — the application forwarding some paths on to a service
the browser never reaches directly — is a different job with a different page:
[Proxying to a Backend Service](/guides/backend/service-proxy/).

## The configuration, both sides

Two files have to agree. On the application side:

```toml
[server]
port = 8080
trusted_proxies = ["127.0.0.1"]   # where the proxy connects from

[security.headers.hsts]
enabled = true
max_age = "8760h"                 # one year; durations have no day unit

[security.csrf]
enabled = true
trusted_origins = ["https://app.example.com"]   # the origin the browser shows
```

And on the proxy side, with nginx on the same host:

```nginx
server {
    listen 443 ssl;
    server_name app.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;

        proxy_set_header Host              $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;

        proxy_http_version 1.1;
        proxy_buffering    off;
        proxy_read_timeout 15m;
    }
}
```

Every line in that `location` block is load-bearing, and the rest of this page
is what each one is holding up.

## What `trusted_proxies` decides

One thing, today: whether `X-Forwarded-Proto` is believed when the security
headers middleware asks if this request arrived over HTTPS. Since HSTS is sent
only on a verified HTTPS request, and a terminating proxy leaves no TLS
connection to verify, the list is what stands between a configured
`Strict-Transport-Security` and a header that never ships. Configure HSTS,
deploy behind a proxy, watch the header not appear — that is this, and the fix
is the list.

It goes no further than that. The framework reconstructs no client IP, and the
other forwarding headers are read nowhere in a production build:

| Header | What reads it |
| --- | --- |
| `X-Forwarded-Proto` | the security headers middleware, gated by this list; and `plugin/auth`, ungated, when it builds the post-logout redirect URL |
| `Host` | the CSRF origin check, and every absolute URL the framework builds |
| `X-Forwarded-For`, `X-Forwarded-Host`, `Forwarded`, `X-Real-IP` | nothing outside development |

The ungated read in that first row is why the proxy must **set**
`X-Forwarded-Proto` rather than append to whatever arrived. `$scheme` in nginx
overwrites; a configuration that preserves a client-supplied value lets the
client name the scheme.

Values are IP addresses or CIDR blocks, and a malformed one fails startup rather
than being skipped. Name the narrowest thing that is true — `127.0.0.1` for a
proxy on the same host, the pod or subnet CIDR for a sidecar or an ingress.

**When the proxy has no fixed address** — an application load balancer drawing
from a rotating pool, a CDN, a platform that hands you no range at all — the
better answer is usually to stop sending HSTS from the application and let the
load balancer send it. That layer terminates the TLS connection, so it knows the
answer the application is trying to infer, and one layer setting a header beats
two layers disagreeing about it. Reach for `trusted_proxies = ["0.0.0.0/0"]`
only when the port is genuinely unreachable except through the balancer, and
write it knowing it says the network is your only boundary.

## The origin check has its own list

This is the failure that arrives as a mystery. The application works on
localhost, goes behind the proxy, and every form submission comes back `403`
with nothing in the response saying why.

The [CSRF check](/guides/architecture/security/#how-csrf-works-here) compares
the browser's `Origin` against the origin the process reconstructs for itself,
scheme included, and it reconstructs that scheme from `r.TLS`. Behind a
terminating proxy there is none, so the process decides it is
`http://app.example.com` while the browser is telling it `https://app.example.com`.
Same host, different scheme, no match.

Nothing repairs this by reading `X-Forwarded-Proto`, and the omission is
deliberate rather than an oversight: the reconstructed origin is the value the
comparison is *made against*, so a caller able to assert the header would be
asserting the answer. A deployment names its own public origin instead.

```toml
[security.csrf]
trusted_origins = ["https://app.example.com"]
```

Name every origin a browser may legitimately show — apex and `www` if both
serve, and each environment's hostname in that environment's file. Only scheme
and host are compared, so a trailing slash or a pasted path is normalized away
rather than failing to match, and an entry naming no scheme is dropped entirely.

## Host has to survive the hop

`r.Host` is the other half of that comparison, and it is also what the OIDC
post-logout redirect is built from. nginx's `proxy_pass` replaces the `Host`
header with the upstream address unless told otherwise, which turns the
reconstructed origin into `http://127.0.0.1:8080` and breaks both. That is what
`proxy_set_header Host $host` is preventing.

While you are deciding hostnames: the application has to own the root of one.
The browser runtime is served from `/_pw/`, a fixed absolute prefix that no
configuration moves, so a `location /app/` that strips its prefix will serve
pages whose script tag resolves to a path the proxy does not route. Give the
application a hostname, or a path that reaches it unmodified.

## Buffering defeats progressive rendering

A page with [await boundaries](/guides/cross-layer/async-rendering/) commits its
shell immediately and sends each region as it settles;
[live rendering](/guides/cross-layer/live-rendering/) holds one response open for
minutes. A proxy that buffers responses collects all of that and delivers it at
the end. The page is still correct — it simply arrives the way it would have
without any of the machinery.

The framework sends no `X-Accel-Buffering: no`, so nothing overrides the proxy's
default from the response side. Turn buffering off where it is configured, which
for nginx is `proxy_buffering off`, and set `proxy_http_version 1.1` so the
upstream response may be chunked at all.

For a proxy you do not control, the escape hatch is to stop producing responses
it will mishandle:

```toml
[html]
streaming = false
```

That disables live rendering too, since live delivery depends on a document
holding placeholders a buffered render never writes. Documents stay valid and
complete; they just settle before the first byte leaves.

Timeouts are the other half. `live_max_duration` defaults to ten minutes, so a
proxy read timeout below that closes healthy connections and the client
reconnects into the same wall. Raise the proxy above it, as the `15m` above
does, or lower `live_max_duration` to fit the timeout you are given.

## One address, every anonymous visitor

`html.live_max_responses` bounds concurrent live responses per client, and the
client is identified by the authenticated subject when there is one and by
`RemoteAddr` when there is not. Behind a proxy every anonymous visitor shares
one address. The bound stops being four connections per browser and becomes four
for the entire deployment, at which point the fifth visitor watching a public
dashboard is refused.

Which fix applies depends on who is watching. Live pages behind a login are
keyed by subject and need no change at all. Public ones need the bound moved:

```toml
[html]
live_max_responses = 0   # unbounded here
```

Zero removes it, which is only safe if something else limits connections — and
the proxy is the layer that can, because it is the one that still sees distinct
clients.

## What stops working in development

`pw dev`'s test endpoints under `/_pw/test/` and the development JWT relaxation
admit only a caller on the same machine, and they treat the presence of *any*
forwarding header as proof the caller is a relay. There is no opt-out and it is
not a configuration mistake. Once nginx or `docker run -p` sits on the host,
every request in the world arrives from `127.0.0.1`, and the address stops
describing the client; the forwarding header is the one artifact that
distinguishes the two cases. Running `pw dev` behind a proxy takes those
endpoints away, which is the whole intent.

## What the proxy is probably already doing

Response compression is off by default because something in front usually does
it, and encoding a body twice benefits nobody. Leave
[`middleware.compression`](/guides/frontend/compression/) off unless the proxy
is not compressing.

Body limits apply on both sides and the smaller one wins.
`server.max_request_body` defaults to 10 MiB while nginx's
`client_max_body_size` defaults to 1 MiB, so an upload can be refused by the
proxy without your handler running or your access log recording it. Raise the
proxy's limit to match before wondering why the application never saw the
request.

Every key named here is listed in full in the
[configuration reference](/reference/configuration/).
