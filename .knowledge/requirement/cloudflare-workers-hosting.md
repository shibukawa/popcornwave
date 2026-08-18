---
id: requirement:cloudflare-workers-hosting
type: requirement
title: Cloudflare Workers Hosting
---
Popcorn Web targets Cloudflare Workers through a fetch-event adapter to net/http while keeping its application handler unchanged.

```yaml
priority: next candidate after the four targets of decision:serverless-target-scope
host_contract:
  input: Cloudflare fetch Request and bindings
  output: Cloudflare Response
  bridge: convert through an http.Handler produced by pw.Middlewares; no listener and no pw.Run
  packaging: Wasm module plus JavaScript entry loaded by Wrangler
current_candidate:
  package: github.com/syumai/workers
  status: experimental upstream and currently does not build with the Popcorn Web application graph
  important_distinction: this path is Cloudflare JavaScript-hosted Wasm, not the component-model WASI HTTP path deferred by decision:wasi-http-deferred
blocked:
  fact: no supported artifact is emitted today
  unknown_until_reproduced: the first incompatible package, compiler diagnostic, and whether host Go Wasm and TinyGo fail at the same boundary
  policy: do not add an unverified dependency, generated entry point, or deploy command that claims support
unblock_probe:
  application: one minimal Popcorn Web handler wrapped by pw.Middlewares and github.com/syumai/workers Serve
  matrix:
    - current project Go with GOOS=js GOARCH=wasm using the upstream worker-go template
    - decision:tinygo-042-baseline or later using the upstream worker-tinygo template
  local_runtime: Wrangler dev receives GET, POST body, duplicate response headers, cookies, redirect, and a streamed response
  limits: emitted upload size is checked against the active Cloudflare plan rather than copied as a permanent framework constant
delivery_after_unblock:
  - a generated Cloudflare entry point that application code does not edit
  - pinned and diagnosed compiler, adapter, wasm_exec, and Wrangler versions
  - api:cli-generate before the Wasm compile, per rule:container-build-inputs
  - wrangler configuration and JavaScript loader generated as deployment artifacts
  - a local conformance test before any remote deploy action
  - documentation for unsupported filesystem, socket, database, streaming, and instance-lifetime behavior
fallbacks:
  - fix or contribute the smallest incompatibility upstream when github.com/syumai/workers remains the right bridge
  - own a narrow fetch-to-net/http adapter only if the required surface is small, testable, and upstream cannot accept it
  - keep Cloudflare as CDN and proxy to a supported container host; this is operational support, not Workers runtime support
non_goals:
  - silently falling back to a different application runtime
  - implementing Cloudflare bindings before the HTTP conformance probe passes
  - claiming generic WASI support from one Cloudflare-specific adapter
acceptance:
  - both compiler paths have recorded pass or precise failure results
  - at least one path builds, runs under Wrangler, and passes the HTTP conformance probe
  - the generated adapter initializes framework resources once per Wasm instance
  - a failed build names the incompatible dependency and supported version matrix
```
