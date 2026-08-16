---
id: decision:cors-above-the-refusals
type: decision
title: CORS Marks The Response Above Every Refusal
---
requirement:cors-middleware is answered by the policy:security-response-headers frame, moved from slot 60 to 52 between SlotRecover and SlotRateLimitProcess, because a response the browser will not hand to script is a status nobody can read, and every frame that refuses a request sits below that number.

```yaml
status: decided 2026-08-13 with the requirement, revised the same day to one frame, implemented the same day
one_frame:
  decided: 2026-08-13; CORS is not a new frame beside the response-header frame, it is more of what that frame already does
  supersedes: an earlier shape in this decision that added a SlotCORS at 52 and left SlotSecurityHeaders at 60
  slot: SlotSecurityHeaders becomes 52; no slot is added
  as_built: the constant moved in pwruntime chain, both transports install the frame when either half is enabled, and the pw test that asserted the ceiling sat between recover and the headers now asserts the headers sit above the ceiling
  why_it_is_the_same_frame:
    both_are_browser_policy: one sets what the browser may render and embed, the other sets what it may read and from where; both are resolved from data:security-runtime-config, validated at startup, and written before commitment
    both_wanted_the_same_position: the requirement needed 52 for the marking to survive a refusal, and policy:security-response-headers was already recorded as owing its headers to every error and operational response, which 60 does not deliver
    one_port_instead_of_two: decision:backend-specific-middleware pays per frame per backend, and this is the frame that would otherwise be paid twice for one position
    one_construction_site: both halves are resolved and validated where the frame is built, so a misconfiguration of either is an error before the port is bound; as built that is ResolveCORS beside ResolveSecurityHeaders rather than one function, for the reason the implementation block of requirement:cors-middleware records
  what_moving_60_to_52_also_fixes: a 429 from SlotRateLimitProcess, and a 503 beside it, now carry the configured policy headers, which the earlier arrangement left off every refusal written above 60
  cost_of_the_move:
    what: an application middleware registered in the 52 to 59 gap ran outside the response-header frame and now runs inside it
    who: nobody in this tree, and any application that picked a number there meaning after the headers keeps its meaning while one meaning before them silently inverts
    stated_because: the numbers are a public surface and pwruntime chain says the order is the part that must not differ, so a moved constant is a change an upgrade note owes its reader
  frame_name: stays security_headers, because the name already means the browser-policy frame and renaming it costs the startup summary, the diagnostics, and every test that names it more than the wider meaning is worth
  preflight_carries_them_too: the 204 this frame answers gets the configured header set like any other response, per the rule that policy:security-response-headers applies to operational responses
forcing_fact:
  browser: script reads neither the body nor the status of a cross-origin response carrying no Access-Control-Allow-Origin, whatever the status was
  framework: the refusals are written by frames, not by the application; 429 at SlotRateLimitProcess, 413 at SlotMaxRequestBody, 401 at SlotAuthentication, 403 at SlotCSRF and SlotGuard, 500 at SlotRecover
  consequence: a frame that marks the response has to have run before all of them, and requirement:typed-http-contract answers every one of those cases with a typed problem the caller was meant to act on
mechanism:
  fact: w.Header() is one map for the whole chain, so headers a frame sets before calling next are on the response every frame below it writes
  panic_case: the recover frame at 50 is outside this one and writes its 500 after this frame returned, and the marking is still in the map, so a panic-derived 500 is readable too
  therefore: marking early costs nothing and covers cases the frame never sees
slots:
  chosen: SlotSecurityHeaders, moved from 60 to 52
  above_it_and_therefore_marked:
    SlotRateLimitProcess 55: the 429 whose Retry-After requirement:rate-limit-problem-responses exists to deliver, and which the move also gives the configured policy headers
    SlotRequestTimeout 70 and SlotMaxRequestBody 80: the two bounds a client is expected to adapt to
    SlotPublicAssets 90: the font and asset case, which needs the header on a success rather than on a refusal
    SlotOperational 100: health and readiness, reachable cross-origin by a dashboard
    SlotSession 120, SlotAuthentication 130, SlotRateLimit 135, SlotCSRF 140, SlotGuard 150: every credential-shaped refusal
    SlotAPIDoc 160: the generated document, whose plausible reader is a UI on another origin
  below_it_and_therefore_still_covering_the_preflight:
    SlotTracing 10: the preflight opens a span like any request
    SlotResources 20: the frame reads its resolved configuration the way every other one does
    SlotClientAddress 25 and SlotRequestID 30: a preflight answer carries a request ID and a resolved caller
    SlotAccessLog 40: the preflight appears in the log, so an operator can see one being answered
    SlotRecover 50: a panic parsing a malformed Access-Control-Request-Headers is a 500 rather than a dropped connection
rejected_alternatives:
  a_second_frame_beside_the_security_headers_at_60:
    shape: two frames, both browser policy, both set before commitment, both left where the header one already sat
    why_not: 60 is below SlotRateLimitProcess, so a rate-limited cross-origin caller receives an unreadable 429 and keeps retrying at the rate that limited it
    what_survived_it: the observation that the two belong together, which one_frame above adopts by moving the pair to 52 rather than by leaving them at 60
    what_did_not: the asymmetry this entry once rested on, that 60 costs the security headers only a header on an error page and costs the marking the entire answer; it is a reason not to sit at 60, not a reason to sit apart
  below_the_process_rate_limiter_at_57:
    shape: preflights counted against the process bound
    why_not: the same 429 problem, for a bound whose whole purpose is to tell a client when to come back
    accepted_cost_instead: a preflight answer escapes the process bound; it reaches no application code, allocates a header set, and the request that follows it is still counted
    settled: 2026-08-13, confirmed rather than left open; the frame takes no bound of its own either, so a preflight is counted by nothing and the remedy for a flood is the layer in front of the process rather than a second limiter inside it
  answering_the_preflight_in_the_router:
    shape: an OPTIONS route per path, or the router's own OPTIONS handling
    why_not: pwfast servemux sets HandleOPTIONS false deliberately, matching Go, so this would reverse a decision taken for the framework as a whole to serve one middleware
    also: a preflight carries no credentials, so a route reached below SlotAuthentication answers 401 to a browser asking whether it may send the real request
    also: a per-route declaration is the per-route thing that gets forgotten, and the forgotten case fails in the browser rather than in a test
  leaving_it_to_the_reverse_proxy:
    shape: nginx or the ingress sets the headers
    why_not: api:cli-dev has no proxy in front of it, so the developer building the cross-origin client meets the failure first and locally, with nothing in the framework to configure
    also: the proxy cannot answer a preflight for a path whose method set the router owns, so it either allows more than the application does or refuses what it allows
consequences:
  - no slot is added; one constant in pwruntime chain changes value, and both transports read it, so the position cannot differ between them
  - policy:security-response-headers gains the refusals written between 52 and 60, which is the gap that concept's own rule about error responses had left open
  - the frame runs on every request of a deployment that enabled either half, including same-origin ones, where the CORS work is one absent-header check
  - a preflight is answered without the session, the authentication or the guard ever running, which is the same reason decision:report-endpoint-above-the-session gives for its own endpoint and reaches the same conclusion from the opposite direction: that one had credentials it must not act on, this one has none to act on
  - the marking is present on responses this frame never inspected, including one written by the recover frame outside it
```
