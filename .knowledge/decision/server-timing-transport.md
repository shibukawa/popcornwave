---
id: decision:server-timing-transport
type: decision
title: Server-Timing Travels As A Header, Not A Trailer
---
requirement:dev-request-timing-surface carries its metrics in an ordinary response header and leaves the transport of api:html-response alone, because a trailer is read by no browser on the wire api:cli-dev actually serves.

```yaml
status: accepted 2026-08-10
proposal:
  what: api:cli-dev always uses chunked encoding and sends Trailer Server-Timing, so timing measured by tracing returns in the response
  right_about: the destination. The timing belongs in the browser beside the request, which is where the developer already is when a page is slow, and requirement:dev-telemetry-viewer holds it one pane away instead
  wrong_about: the vehicle, for the reasons verified below
verified_2026_08_10:
  trailers_are_devtools_only:
    fact: only a browser's DevTools may read Server-Timing from a trailer; the Fetch API cannot access trailers at all
    source: the explicit warning on the MDN Server-Timing page
    consequence: nothing sent as a trailer reaches PerformanceServerTiming, so no page script and no pane can ever read it
  chrome_reads_none: Chromium has declined trailers repeatedly as too large a change to interposed network APIs; Chrome does not detect a Server-Timing trailer and does not show one
  firefox_reads_them_over_https_only:
    implemented: bugzilla 1413999, fixed in 59 and shipped in 64, which was the first time any browser exposed a trailer in any form
    restricted: bugzilla 1436517 limited it to HTTPS, and Firefox signals trailer support only over HTTP/2, which it runs only for https origins
  the_dev_wire_is_cleartext_http1:
    fact: api:cli-dev binds a plain TCP port and no path in it configures TLS, which decision:development-port-shift restates in the address it reports
    therefore: on the transport pw dev serves, a Server-Timing trailer is read by zero browsers. Chrome ignores it, and Firefox rejects it for not being HTTPS
  the_header_form_works_everywhere:
    exposure: PerformanceServerTiming has been baseline since March 2023 and appears in the DevTools timings tab
    secure_context: the interface is gated on a secure context, and http://localhost is one, so the gate is already satisfied without TLS
decision:
  - emit Server-Timing as a response header written at commit, on both branches of decision:automatic-async-render-selection
  - send no trailer, and declare no Trailer header
  - change no transport framing; whether a response is chunked stays a property of what policy:response-content-encoding and the branch already decided
  - answer the post-commit half from requirement:dev-telemetry-viewer, correlated by the trace id the header carries, rather than from the wire
what_the_header_cannot_carry:
  fact: headers are written at commit, so a streamed response can state only what happened before its first byte
  kept: that is the time to first byte broken into its phases, which is the part no other tool on the machine reports and the part a stopwatch cannot separate
  lost: boundary settle, live delivery, and every statement after commit
  recovered: those are spans in data:framework-span-set already, held by the viewer, reached by the trace id
  why_that_is_not_a_workaround: the trailer would not have recovered them either, because nothing in the browser can read one here; the choice was never between two working delivery paths
why_not_force_chunked_in_dev:
  it_bought_only_the_trailer: with the trailer gone there is no remaining consumer, so the transport change answers nothing
  it_costs_parity: a dev-only framing means the developer loop stops rehearsing what is deployed, which is the failure requirement:deployed-debug-information names when it insists a staging artifact match a prod one
  the_scar_is_on_record: api:html-boundary-protocol carries a parser bug that was invisible in development and surfaced only once a proxy, TLS record, or compressing encoder split the bytes. Divergence in how dev frames a response is the shape that already cost this project once
  already_half_true: an encoded response drops Content-Length today, so a dev run with compression on is length-unknown without anyone deciding it; forcing the rest would make the exception the rule for no reader
  fasthttp_has_no_streaming_branch: api:pwfast-package renders into a buffer and commits after it succeeds, so a framing rule justified by streaming means nothing on that half. Its fork can write trailers, which makes this a shape mismatch rather than a capability gap
adjacent_and_not_taken:
  what: deliberately splitting a streamed response at hostile points under a development switch, so the marker invariant of api:html-boundary-protocol is exercised where it is cheap to fix
  why_it_is_the_real_version_of_the_idea: that bug was found in production framing and not in dev, and adversarial splitting is the transport knob that would have caught it
  why_not_here: it is a fault injector rather than a timing surface, it is opt-in rather than always on, and it shares nothing with this requirement but the word chunked
consequences:
  - one gate and one code path serve every environment, since a header needs no build tag and no framing rule
  - a non-browser client reading trailers gains nothing, which costs nothing because no such reader exists here
  - reopening the trailer needs only one fact to change, that a browser reads one over cleartext HTTP/1.1, and nothing else in this decision depends on the rest
references:
  - https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Server-Timing
  - https://bugzilla.mozilla.org/show_bug.cgi?id=1413999
  - https://developer.mozilla.org/en-US/docs/Web/API/PerformanceServerTiming
```
