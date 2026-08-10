---
id: decision:forwarded-header-trust
type: decision
title: A Forwarded Header Is Read Only Behind A Declared Proxy
---
A forwarded header is read when the peer address falls inside server.trusted_proxies and is treated as absent otherwise; a declared origin remains what an origin comparison matches against.

```yaml
status: proposed
serves: requirement:proxied-request-identity
the_two_positions_this_reconciles:
  refuse:
    held_by: policy:csrf-protection
    said: forwarded headers are not trusted, and a proxied deployment names its origin instead
  gate:
    held_by: policy:security-response-headers
    said: emit HSTS only for an effective HTTPS request after trusted-proxy evaluation
  both_were_right_about_different_questions:
    comparison: what an origin check may accept, where a declared origin is strictly better than any header
    reconstruction: what this request's own scheme is, which has no declared answer to fall back to
  what_conflating_them_produced: a third reading, ungated, in the logout path of api:authentication-endpoints, written because neither existing position offered an answer to the reconstruction question
the_rule:
  precedence: r.TLS outranks every header, since a directly served TLS request needs no assertion
  gated: X-Forwarded-Proto and X-Forwarded-For are read when the peer is inside a configured network
  ungated: never read; a header from an untrusted peer is treated as absent rather than as false, so a missing trust configuration degrades to the pre-proxy answer instead of inverting it
  forwarded_rfc7239: not read, because no deployment target of decision:local-tls-proxy-boundary emits it by default and a second grammar is a second parser to keep correct
  x_real_ip: not read, for the same reason; X-Forwarded-For is what the targets emit
why_a_gate_rather_than_keeping_the_refusal:
  fact: the refusal was written when the only caller was an origin comparison
  and_there_it_still_holds: a declared origin is an answer the deployment already had to supply for another reason, so reading a header would trade a certainty for an assertion
  but: an effective-scheme question has no declared counterpart, and refusing the header there does not produce a safe answer, it produces http on an https deployment
  evidence: both other readings grew independently, one gated and one not, which is what an unanswered question looks like in a tree
what_the_gate_does_not_buy:
  origin_acceptance: a gated header never makes an undeclared origin acceptable; it decides this request's own scheme and nothing about the peer's
  spoofing_within_trust: a proxy inside the trusted set is trusted completely, which is the assumption decision:local-tls-proxy-boundary already makes about the application hop, not a new one
  header_stripping: this framework does not strip inbound X-Forwarded-* on an untrusted request; it ignores them, because a handler that never reads an ungated value does not need the header gone
consequences:
  - server.trusted_proxies becomes a security-relevant setting on every proxied deployment, rather than a header-handling detail of one middleware
  - an unset value leaves a working but degraded deployment, which is why rule:configuration-advisories reports it and startup does not refuse it
  - policy:csrf-protection keeps its declared-origin requirement unchanged, so no deployment loses a check by this
  - the development refusal is unaffected, because a development listener has no trusted network configured and the ungated clause is what already governs it
```
