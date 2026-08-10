---
id: requirement:alternate-http-backend-readiness
type: requirement
title: Readiness For A Non-net/http Backend
---
Adding a second HTTP backend must stay a framework-sized change: paid once inside pw and the generated-code templates, never once per application handler.

```yaml
status: proposed
not_a_commitment:
  planned: no fasthttp backend is scheduled for a release
  committed: keeping the option cheap, because losing it is paid silently over every handler written meanwhile, while keeping it is one rule
  published_position: the why-popcorn-wave page says fasthttp is a different boundary and this framework will not sit under a net/http-shaped compatibility layer; readiness does not contradict it, because this is about what application code names, not about shipping an adapter
what_a_swap_actually_costs:
  handler_shape: fasthttp dispatches func(*fasthttp.RequestCtx), one value carrying both directions, so a two-parameter net/http handler has no shape-preserving translation
  context: '*fasthttp.RequestCtx implements context.Context itself, so nothing wraps it and r.Context() has no counterpart there'
  lifetime: that value and everything reachable from it are pooled and invalid once the handler returns, which net/http never required and application code therefore does not respect today
  adapter: an adaptor converts a net/http handler by materializing the net/http values the backend exists to avoid allocating, so it answers compatibility and not the reason anyone switches
all_or_nothing_now:
  fact: decision:transport-compatibility-fallback records the reversal; there is no adapter, and a function the rewriter refuses fails the build
  effect: this requirement stopped being about keeping an option cheap and became the precondition for the option existing at all
  visible_before_committing: the upstream report-only run lists every refusal without generating, so the whole cost is knowable in advance rather than discovered during a migration
why_the_rule_pays_regardless:
  - decision:wasi-http-deferred already names this property as its future boundary, and a component-model host is the likelier payoff than fasthttp
  - a handler naming the transport only through pw is the handler api:cli-generate can analyze, per requirement:httpbinder-extensible-route-analysis
  - containment is testable now; a backend port is not
mechanism: decision:transport-handle-containment
acceptance:
  - every transport-typed call shape reachable from application code is enumerable from the pw surface
  - scaffolds from api:cli-init, the examples, and the tutorial name w and r only at that surface
  - a hypothetical backend changes pw, pwruntime, middlewares, and the generated-code templates, and no application file, except the application's own middleware, which decision:backend-specific-middleware never promised
  - the upstream report-only run over every scaffold, example, and tutorial package reports nothing
largest_item: the full framework middleware set, ported per backend rather than adapted, per decision:backend-specific-middleware
prerequisites:
  surveyed: 2026-08-09, against tinybind-go v0.4.10
  already_neutral:
    htmlbind: every render entry point takes io.Writer, so the heaviest upstream dependency needed no port; what is net/http-shaped is the negotiation, status, header, compression, and flush layer around it, which is pw's own code
    jsonbind: an append-to-bytes API with no HTTP in it
    sqlbind, configbind, minitoml, cliparser, dynamobind, firestorebind: no HTTP
    internal path grammar: string matching, shared by the path-scoped policies
  upstream_status: requirement:tinybind-alternate-backend-support, which records what shipped and what is left
  router:
    default: the tinygodriver fasthttprouter fork, not the upstream one, and the reason is a type identity rather than a preference
    why: the upstream router takes valyala fasthttp's request value, while the TinyGo fork vendors its own, so a handler built against the fork is not the type that router accepts; upstream vendored the router beside the fork instead of giving up the TinyGo build
    configurable: a router target names the import, qualifier, type, registration function, and catch-all spelling, so an application on upstream fasthttp points it at the upstream router and nothing else moves
    grammar: named parameters carry over verbatim in both directions, and only the catch-all spelling is rewritten
    no_catch_all: a target declaring no catch-all spelling rejects such a route by name rather than inventing one
    matching_semantics: still the risk a transform cannot rewrite, so rule:route-and-template-checks comparing data:route-table across both builds is unchanged by any of this
  settled_2026_08_10:
    build_tag_axes:
      answer: independent axes, so the backend is not pinned to the TinyGo target
      but: TinyGo plus fasthttp is tier two, compiled and kept compiling but excluded from performance comparison, because that combination is both larger and no faster today
      primary_target: host Go, which the measured 1.44x difference in CPU per request gives its own reason for, independent of anything TinyGo needs
      consequence: the paired files gain a third variant only where the backend genuinely reaches them, and the tier-two rule keeps the fourth quadrant a compile check rather than a supported configuration
    pw_in_the_second_binary:
      answer: absent, a clean split
      consequence: everything transport-free that pw owns has to reach a shared package before the second build can start at all, which is configuration binding, session, the database layer, and observability
      why_not_the_cheap_one: a mixed binary needs no moving and carries the whole net/http stack it never serves with, which gives up the binary size the split is for and leaves two homes for every later decision
      staging: the mixed shape works today and is a legitimate intermediate, so the move is per layer rather than all at once; what the answer settles is the destination, which is what decides where a thing is put when it is touched
    test_seam:
      answer: built first, before more porting
      shape: pwtest holds the neutral request and response, and each transport supplies one Exchange with the same name and signature, so a test moves between them by changing its import
      counted: 86 test files drive handlers through httptest here, 66 of them by building a request and reading a recorder, which is the population that would otherwise have been written twice
      real_server: both halves run a real server over an in-memory pipe rather than a recorder, because half of what is worth testing is decided by the transport and a hand-built request value tests the entry against the test's idea of it
    dev_tooling_scope:
      answer: out of scope, as proposed
      covers: the development console, the identity provider, the telemetry viewer, and the storybook
      why: each is a host-side tool standing up its own server rather than part of the application serving path
  identity_endpoints_2026_08_11:
    was: the largest single item left, and the one this document called too security-sensitive to rush
    answer: not a port; plugin/auth grew auth.Exchange, a transport seam, and every endpoint body was rewritten against it, so both transports drive one implementation of the login
    what_the_seam_carries: the request line, one query parameter, one form field, one header and all of one header, a bounded body, the three connection facts the origin rules read, and the four response operations
    why_not_two_ports: decision:backend-specific-middleware says port the framework middleware set per backend, and this is the boundary where that stops paying; a middleware frame is a few rules, and a login is the accumulated judgement of three protocols
    supporting_moves:
      session: Manager.AttachTo, RotateOn and DestroyOn, plus Jar.LoadFrom
      pwruntime: StoreAuthentication, the write half of the reader that was already portable
      guard: auth.Rules, the resolved protection policy as a value, which pwfast.GuardPolicy already had the shape for
    fasthttp_half: popcornwave/plugin/auth/authfast, about two hundred lines, of which the exchange is most
    all_three_modes_serve: oidc, passkey and jwt_only, each covered by an end-to-end binary against a real provider and a real database
    agreement_test: both listeners are asked the same question over one runtime and the answers are compared, which found a real divergence in Redirect
    still_mixed: authfast links plugin/auth and therefore pw, because the session manager and the extension registry have not moved; the configuration layer, which this entry named as the blocker, is done — see configuration_layer_moved_2026_08_11
  configuration_layer_moved_2026_08_11:
    first_of_four: pw_in_the_second_binary lists configuration binding, session, the database layer and observability as what has to reach a shared package; this is the first
    package: pwconfig, holding the registry, the load, the framework's own bindings and their generated definitions, the environment the load resolves against, and the connection group resolution
    the_earlier_reading_was_wrong: pwruntime's configlookup argued that moving the registry would drag registration, defaults, the environment overlay, scaffold emission and the boot report with it, so the read was published instead; only two of those are runtime-shaped, and both are hooks now
    what_stayed_a_hook: the argument filter that takes framework subcommands off the command line, and the callback the startup summary reads
    chain_settings_followed: the reduction into pwruntime.ChainSettings moved too, so a parse publishes what a chain builder needs; without that a pw-free build parsed a file and then had pwfast.Middlewares refuse for want of settings
    application_surface_unchanged: every type is a true alias and every entry point a thin wrapper, so no application, scaffold or document changed
    proved_by: internal/fastonly, a real package that parses a configuration file and serves one fasthttp request through a chain composed from what it read, whose dependency graph is asserted to contain no pw
    what_is_left: nothing of the four; see all_four_layers_moved_2026_08_11
  all_four_layers_moved_2026_08_11:
    layers:
      pwconfig: the registry, the load, the framework's own bindings, the environment, and the connection group resolution
      pwdatabase: opening the configured pools, the group selectors, and the pool lifetime
      pwsession: the slot registry, the backend registry and its factories, the keyring, the cookie policy, the lifetime arithmetic, and the expiry sweep
      pwobservability: the log backend, the OTLP exporters, the query diagnostics, and the span policy
    what_each_runtime_kept: one frame per layer, and nothing else — manager.Middleware against pwfast.Session, the startup summary that reads what was resolved, and the shutdown order, which is a property of the chain that was built rather than of any layer
    stacking: pwdatabase over pwconfig, pwsession over both, pwobservability over pwconfig, and no edge back; asserted rather than intended, because a cycle here is a startup order nobody could state
    application_surface_unchanged: every moved type is a true alias and every moved entry point a thin wrapper
    proved_by: go list -deps over all four, plus internal/fastonly, which parses a configuration file and serves one fasthttp request in a build whose graph contains no pw
  plugin_freed_2026_08_11:
    was: plugin/auth linked pw for two things, and authfast through it — the extension registration in auth.go and the net/http Exchange in httpexchange.go, which reached pw.WriteProblem and pw.Redirect
    the_split_that_was_not_needed: the plan was plugin/auth/authcore plus 144 aliases in plugin/auth, keeping the documented blank import; what made it unnecessary is that the two things are small and neither needs the runtime, only net/http
    answer: pwextension, a leaf holding the net/http extension registry and the two responses a plugin writes, published by whichever runtime is linked
    net_http_is_not_a_runtime: the leaf names net/http, which is a protocol library; what is worth not linking is the framework built on it
    no_counterpart_on_the_second_transport: pwfast.Middlewares takes its frames as arguments and reads no registry, so this registry is the net/http chain's and a plugin hands the other transport a frame directly
    honest_defaults: with no runtime published, Problem writes the document without the error page and Redirect falls back to net/http's helper; the status, headers and body are all still there and only the presentation a browser would have preferred is not
    fell_out_of_it: the problem document was built by hand in both runtimes and is now pwruntime.AppendProblemJSON, so one response body is described once
    cost: no aliases, no package split, and the documented import line is unchanged
    proved_by: authfastjwte2e, which starts through pwconfig, plugin/auth and pwfast with no pw in its graph and asserts that, and go list -deps over plugin/auth and authfast
  open_here:
    superseded_note: the four entries below were the open list; all four are answered in settled_2026_08_10 above and are kept here for the reasoning rather than as questions
    test_seam:
      fact: 77 files in this repository drive handlers through httptest, and the other backend tests through an in-memory listener instead
      risk: without a backend-neutral seam in the test utilities first, every one of those tests is written twice
      owner: api:test-run and decision:testutil-testing-interface
      rank: highest leverage of anything on this list, and the easiest to skip
    build_tag_axes:
      today: two axes already, the TinyGo target and decision:force-tinygo-logic, visible as the paired files in pw
      question: whether the backend is a third independent axis or is pinned to the TinyGo target, which decides whether every paired file needs a third variant
      sharper_now: the upstream fork exists because the fasthttp backend is aimed at TinyGo, which suggests the two are not independent, and settling it decides the size of the matrix
    dev_tooling_scope:
      fact: the development console, the identity provider, the telemetry viewer, and the storybook each construct their own net/http server and are host-side tools rather than the application serving path
      proposal: declare them out of scope, so the port does not grow to include them
    handler_file_layout: a transport handler must sit in a file of its own, per decision:transport-source-transform, which the scaffolds and examples must be made to satisfy
  do_first_here:
    what: register the pw surface per requirement:pw-call-registration, then run the upstream report-only generation over the examples, scaffolds, and tutorial
    why: it produces the actual work list rather than an estimate, and a clean run is the definition of an application that can adopt the backend
    cost: low, and the registration is useful whether or not a backend is ever shipped
non_goals:
  - shipping a fasthttp backend
  - hiding http.ResponseWriter and '*http.Request' from handler signatures, which decision:root-pw-api deliberately keeps visible and testable
  - a compatibility layer of any kind, which upstream specified and did not implement
```
