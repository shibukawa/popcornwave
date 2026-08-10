---
id: data:rate-limit-runtime-config
type: data
title: Rate Limit Runtime Config
---
The `ratelimit` binding declares one window, the counts allowed inside it, and where the counters live.

```yaml
registration: automatically registered by pw
enforces: requirement:rate-limit-enforcement
own_binding:
  chosen: a top-level section, because the store fields are the same weight as the ones that gave data:session-runtime-config its own
  considered: security.rate_limit, alongside csrf and headers; rejected because that binding is request-forgery and response headers, and a backend selection with a DSN does not belong beside a header list
fields:
  enabled: bool
  backend: memory or redis
  window: duration over which every count below is measured
  per_subject: requests one authenticated subject may make in a window; zero disables this bucket
  per_address: requests one caller with no session may make in a window; must be positive, and at most process when that is set
  process: total arrivals allowed in a window, unkeyed; zero leaves only the identity buckets and leaves a distributed flood to the edge alone
defaults:
  enabled: false
  backend: memory
  window: 1m
  per_subject: 600
  per_address: 300
  process: 0
plugin_fields:
  ratelimit/redis:
    redis.dsn: redis or rediss URL, read from RATELIMIT_REDIS_DSN or a ${NAME} reference, classified secret like every other DSN
    redis.key_prefix: string, default pw:ratelimit:, isolating these keys from every other user of the server as data:session-runtime-config does
    redis.connect_timeout: duration bounding the startup ping and per-command deadlines
two_layer_flood_defence:
  per_address_is_the_floor:
    rule: it is always bounded and has no off position, unlike per_subject
    why: it is the only bucket an unauthenticated flood meets, so an unlimited value is not a permissive configuration but an absent control
    bounded_above_by: process, since one address allowed more than the total ceiling is a limit that cannot bind
  what_per_address_cannot_see:
    fact: a distributed flood keeps every source under the per-address count by construction, so no per-address value catches it at any setting
    consequence: the residual is the process ceiling's, which is what makes that key load-bearing rather than an extra
  and_still_not_the_first_line: the edge drops a flood before it costs a connection, per the division_of_labor of requirement:rate-limit-enforcement; both keys here are depth behind that, for the deployment whose edge rule is absent or wrong
  sizing:
    process: against what this deployment can serve, not against an attack, since a ceiling below real capacity refuses legitimate traffic globally and is the most dangerous value in this binding
    consequence: it defaults to zero rather than to a guess, and rule:configuration-advisories reports an enabled limiter that left it there
why_two_identity_counts:
  fact: the identity key of requirement:rate-limit-enforcement is a subject where there is one and an address otherwise, so one number would govern two populations
  authenticated: accountable and revocable, so a generous allowance costs little
  anonymous: the abuse surface, and also the bucket a corporate NAT shares among many real people, so the number is chosen against both pressures rather than either
  not_a_third_mechanism: one bucket, one window, one store operation; the count is selected by which kind of key resolved
window_is_configurable:
  why: the algorithm is a fixed window, so this value is the burst granularity itself, and a hard-coded minute would send every other shape to an API gateway
  wire: it is also what X-RateLimit-Reset reports, per requirement:rate-limit-problem-responses, so it is already a value the client sees
not_configured:
  exclusions: the operational endpoints and public asset carve-out is fixed, per the fixed_exclusions of requirement:rate-limit-enforcement
  per_route: delegated to an API gateway, so no pattern list exists here
  store_failure: fail open is not a switch, per the failure_mode of requirement:rate-limit-enforcement
  retry_after: derived from the window reset rather than declared
failure_thresholds_live_elsewhere:
  keys: data:authentication-runtime-config declares how many failed attempts an account or session may accumulate and over what period
  why: the extension clause of rule:configuration-advisories puts a plugin's keys in the plugin's own binding, and only the authentication endpoints know what counts as a failure
  supplied_here: the counter those keys are evaluated against, which is the same store this binding configures
rules:
  - a positive count with backend memory is valid and enforces per replica, which rule:configuration-advisories reports
  - zero per_subject disables that bucket, because an authenticated caller is accountable and revocable by other means
  - zero per_address is rejected at startup; the address bucket has no off position, per two_layer_flood_defence
  - a positive process must be at least per_subject and per_address, since one caller allowed more than the total ceiling describes a limit that cannot bind
  - redis fields are rejected at startup under backend memory, rather than being ignored
  - the DSN is classified secret, so policy:log-emission redaction and the literal-secret-in-config-file check of rule:configuration-advisories apply without naming the field
```
