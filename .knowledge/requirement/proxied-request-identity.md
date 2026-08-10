---
id: requirement:proxied-request-identity
type: requirement
title: Proxied Request Identity
---
A deployment behind a TLS-terminating proxy resolves one effective scheme and one client address per request, through one surface every caller shares.

```yaml
status: proposed
premise:
  posture: decision:local-tls-proxy-boundary terminates TLS upstream, so the listener sees the proxy rather than the client; this is the normal deployment, not an edge case
  stronger_than_that: decision:ingress-tls-termination records that no TLS ingress exists in the serving path at all, so r.TLS is nil on every request of every deployment started through a framework entry point
  consequence: the r.TLS branches in requestorigin and in the policy:security-response-headers middleware are unreachable today, which is why an implementation reading r.TLS alone is not a conservative default but a constant
scope: effective scheme, request origin, and client address
out_of_scope: the host, because no caller reconstructs one from a forwarded header today, and adding that reading is a separate decision about host-based routing
three_answers_today:
  verified: 2026-08-10, by reading the tree
  requestorigin:
    reads: r.TLS only, refusing forwarded headers by design
    behind_a_proxy: reconstructs http for an https browser, so its self-origin never matches the browser's Origin
  security_headers:
    reads: X-Forwarded-Proto, gated on the peer being inside server.trusted_proxies
    consumer: the HSTS rule of policy:security-response-headers
  auth_logout:
    reads: X-Forwarded-Proto with no gate at all
    consumer: the post_logout_redirect_uri of api:authentication-endpoints
  contradiction: one refuses the header, one trusts it under a gate, one trusts it ungated; requestorigin's own stated reason for existing is that one question with two answers is one answer that drifts, and the third answer is the drift arriving
consequences_today:
  csrf:
    effect: policy:csrf-protection needs security.csrf.trusted_origins declared by hand on a proxied deployment, or every unsafe request is refused
    visibility: loud, so it is found during the first deployment rather than in production
  auth:
    effect: nothing to configure, because the passkey origin allowlist and the OIDC redirect URL already declare the origin, per the auth_trusted_origins of policy:csrf-protection
    finding: the asymmetry with csrf is unintended rather than designed; the same deployment configures one and not the other for one comparison
  hsts:
    effect: emitted only when server.trusted_proxies is set, and silently absent otherwise
    visibility: silent, which is what makes it an advisory in rule:configuration-advisories rather than a bug report
  logout_uri:
    bound: the ungated read can only raise the scheme from http to https, on a host taken from r.Host, so nothing redirects anywhere new
    defect: it answers differently from the other two for one request, which is the drift rather than the disclosure
  live:
    effect: the client_key of policy:live-subscription-bounds falls back to the remote address, which is the proxy, so html.live_max_responses collapses to one bucket per proxy node and refuses the visitor after the fourth
    scale: this is every anonymous visitor of a proxied deployment, not the NAT corner case that policy recorded
required:
  one_resolver:
    owner: internal/requestorigin, extended with the trusted-proxy evaluation the policy:security-response-headers middleware holds today
    answers: effective scheme, self-origin, and client address
    callers: policy:csrf-protection, api:authentication-endpoints, policy:security-response-headers, policy:live-subscription-bounds, and requirement:contrib-websocket
    removes: the ungated read in the logout path, and the second copy of the trusted-proxy evaluation
    trust_rule: decision:forwarded-header-trust
  wiring:
    fact: the compiled server.trusted_proxies networks reach the security-headers middleware and nothing else
    owed: the same compiled value reaches the CSRF middleware and the authentication plugin
    cost: this is the whole implementation cost of the scheme half; the evaluation itself already exists and is already tested
  client_address:
    rule: walk X-Forwarded-For right to left while the peer is trusted, and take the first address outside the trusted set
    absent_trust: no configured network yields the remote address unchanged, per the ungated clause of decision:forwarded-header-trust
    consumers: policy:live-subscription-bounds, whose bound is currently unenforceable behind a proxy, and requirement:rate-limit-enforcement, which is a hard dependency rather than a later user because an unresolved address turns its anonymous bucket into an outage
    one_computation: both compute the same subject-or-address key today, and the resolver is what stops that becoming a third copy
  advisories: rule:configuration-advisories carries the checks, because the failure mode of every item above is a value nobody set
unchanged:
  declared_origin_wins: an origin comparison still matches against a declared origin; the resolver supplies the self-origin the comparison starts from and never becomes a way to accept an origin nobody declared
  dev_refusal: the forwarded-header refusal in the development paths of api:cli-dev and requirement:contrib-devidp is deliberate and stays, because a development listener has no proxy in front of it and a header there is a caller's assertion
  no_new_config_key: server.trusted_proxies already exists; this requirement changes who reads it, not what a deployment declares
websocket_dependence:
  fact: requirement:contrib-websocket wires its CheckOrigin to this resolution, and its upgrade request carries cookies outside CORS, so the origin check is the only defence on that path
  consequence: every other caller survives a wrong answer with a configuration workaround; that one does not, because the workaround for a rejected upgrade is a permissive CheckOrigin, which is the cross-site hijacking the check exists to stop
  ordering: this requirement precedes that one
```
