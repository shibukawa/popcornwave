---
id: decision:ingress-tls-termination
type: decision
title: Ingress TLS Is Declared, Not Inferred
---
The framework serves plaintext and terminates no inbound TLS today; a deployment that terminates its own declares it in configuration, and nothing infers the boundary from a header or an absence.

```yaml
status: proposed
serves: requirement:proxied-request-identity
distinct_from: decision:local-tls-proxy-boundary, which is about egress to external systems; inbound termination was never decided, only left undone
today:
  verified: 2026-08-10, by reading the tree
  fact: no ListenAndServeTLS, ServeTLS, or TLSConfig exists in the serving path; api:application-lifecycle Serve calls ListenAndServe even when handed a server carrying a TLS configuration
  consequence: r.TLS is nil on every request of every deployment that starts through a framework entry point, so the r.TLS branches of requestorigin and the policy:security-response-headers middleware are unreachable in practice
  escape_hatch: an application taking the handler and running its own server can terminate TLS, at the price of leaving the lifecycle, the validated timeouts, and shutdown behind; that shape is outside what api:cli-doctor can observe at all
why_this_matters_beyond_tls:
  fact: an advisory cannot distinguish a proxied deployment from a directly served one when neither is declared
  effect: every check in the proxy group of rule:configuration-advisories was written as a warning for want of that distinction, which is a weaker finding than the situation deserves
  resolution: make the boundary a declared fact, and the checks become determinate
the_decision:
  declare: server.tls.cert_file and server.tls.key_file; their presence selects ListenAndServeTLS and is the sole signal that this deployment terminates its own TLS
  absent: plaintext, and every consumer of requirement:proxied-request-identity treats the deployment as proxied
  refused:
    acme: no automatic certificate issuance, renewal, or challenge handling; that is what the boundary in front is for, and an ACME client is a subsystem rather than a flag
    redirect: no built-in http-to-https redirect listener, for the same reason
    cipher_policy: the Go default, not a configuration surface
  reload: out of scope for the first release; a certificate change is a restart, which is what a container deployment does anyway
  tinygo:
    status: net/http only
    reason: a TLS server is the class of incompleteness decision:local-tls-proxy-boundary already names on the client side, and policy:contrib-compatibility bounds it
    shape: the keys validate and the entry point refuses on a target that cannot serve TLS, rather than the configuration being silently ignored
why_add_it_at_all:
  small: two keys and one branch at the call site, against a serving path that already centralizes server construction
  buys: a declared deployment shape, which is what turns four warnings into determinate checks and removes the need for a per-advisory off switch
  and: a single-binary deployment with no proxy is a real shape this framework otherwise cannot serve safely, since policy:cookie-value-protection and the session cookie secure default assume https reaches the browser
no_advisory_off_switch:
  considered: a flag silencing csrf-trusted-origins-unset for a deployment that does not need it
  rejected: an off switch records that someone was tired of a message, while server.tls records what the deployment is; the second suppresses the advisory as a consequence and stays true when the deployment changes
  precedent: every other advisory in rule:configuration-advisories keys off a configured fact rather than a mute, and one mute invites the next
```
