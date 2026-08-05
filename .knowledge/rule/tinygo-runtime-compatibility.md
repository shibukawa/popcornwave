---
id: rule:tinygo-runtime-compatibility
type: rule
title: TinyGo Runtime Compatibility
---
Release builds must prove that application runtime code and generated mapping compile with the selected TinyGo toolchain.

```yaml
requirements:
  - installed TinyGo satisfies decision:tinygo-042-baseline
  - tinygo executable is available
  - host Go version is supported by the installed TinyGo version
  - generated artifacts are current
  - application runtime does not import generator packages
  - no runtime reflection-based request mapping
  - configured target supports the application's net/http listener model
  - the target runs the goroutines and range-over-func sequence that requirement:async-html-rendering depends on, or data:html-render-config disables streaming for that build
  - the project registers a Netdever, via the tinygohelper.go blank import of system:tinygodriver netdev
  - a project whose requirement:database-engine-selection engine speaks a network protocol builds with -scheduler=threads
scheduler:
  required_for: requirement:contrib-postgresql and requirement:contrib-mysql
  reason: under the cooperative tasks scheduler a blocking socket call holds the whole runtime, so the driver's cancellation watcher never runs
  symptom_when_missing: a query outlives its context deadline and returns a nil error, with nothing logged
  measured: a 5s server-side sleep under a 500ms deadline returned after the full 5s
  default: threads on desktop targets, so the constraint is an assertion rather than a change for most projects
  applies_to: api:cli-build, api:cli-dev, and every TinyGo test target
wasm_panic_recovery:
  finding: TinyGo's wasm targets do not run a deferred recover at all, measured upstream against a plain function with no goroutine involved
  consequence: a panic in an async external or a requirement:live-html-rendering live source ends the program instead of becoming a boundary error the recover clause renders
  scope: the platform's, not the framework's; system:tinybind names the condition rather than working around it
  handling: a wasm build must treat a panicking external as fatal, so the code behind one belongs under the same review as any other unrecoverable path
unsupported_runtime_packages:
  net/http_client_timeouts:
    behavior: the TinyGo client dials and then reads with no deadline, marked TINYGO TODO handle timeouts in its own source at 0.41.1
    consequence: the context deadlines requirement:contrib-oidc and requirement:contrib-jwt set around discovery, token exchange, and JWKS fetches bound nothing, so a slow or hanging IdP holds the request until the peer closes
    surface: a login or a bearer verification, both of which run on a request handler
    handling: contrib/internal/authn EnforceDeadlines wraps the transport under the tinygo tag and is identity on host Go; both clients apply it themselves rather than leaving it to the application, because they accept a RequestTimeout and therefore promise one
    residual: the round trip beneath cannot be cancelled, since that runtime offers nowhere to cancel it, so a hung peer costs a goroutine and a connection until the socket fails rather than a stalled handler; the abandoned response is drained and closed
    not_reproducible_under: decision:force-tinygo-logic, which selects the TinyGo code paths while still linking the host net/http, so this is one of the few behaviours that has to be tested on the real toolchain
    verified: tinygo test of contrib/internal/authn on 0.41.1 darwin/arm64, found by security review on 2026-08-05
    kind: availability rather than admission; nothing is accepted that would otherwise be refused
  os/signal:
    behavior: registering a handler replaces the default disposition but never delivers to the channel
    consequence: the process stops responding to Ctrl+C and SIGTERM
    handling: api:application-lifecycle installs no handler under the tinygo build tag
    verified: TinyGo 0.41.1 darwin/arm64
netdev_registration:
  file: tinygohelper.go in the project root, package publicassets
  constraint: //go:build tinygo
  import: _ "github.com/shibukawa/tinygodriver/netdev"
  linkage: concept:project-layout bootstrap generator blank-imports the root package
  symptom_when_missing: the binary builds and then exits at startup with "Netdev not set"
baseline_evidence:
  httpbind_go_verified_tinygo: 0.40.1
  compatible_go_range_for_that_baseline: 1.19-1.25
  source: https://github.com/shibukawa/httpbind-go#tinygo
verification: api:cli-build and package target tests
```
