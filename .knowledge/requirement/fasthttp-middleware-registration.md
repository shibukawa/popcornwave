---
id: requirement:fasthttp-middleware-registration
type: requirement
title: Application Middleware Registration On The fasthttp Build
---
An application serving on the fasthttp build must be able to position its own middleware on the shared slot line of requirement:application-middleware-registration without assembling the chain by hand; only the registration seam is required, because decision:backend-specific-middleware already settles that the body is written per transport.

```yaml
status: implemented 2026-08-17, in pwfast/registermiddleware.go
audience: actor:application-developer
serves: requirement:second-build-feature-parity
extends: requirement:application-middleware-registration, whose number line and refusals this reuses unchanged
current_state:
  net_http: pw.RegisterMiddleware(slot, name, middleware) called from main, delegating to the pwextension registry, per requirement:application-middleware-registration
  fasthttp:
    registration_function: none; pwfast exports no RegisterMiddleware and no equivalent
    only_seam: RuntimeOptions.Extra []Frame, documented as what a plugin's frames travel in
    reachable_from_run: pwfast.WithRuntimeOptions(func(o pwfast.RuntimeOptions) pwfast.RuntimeOptions { o.Extra = append(o.Extra, pwfast.Frame{...}); return o })
    verdict: an application can do it and nothing says so; the seam is spelled for a plugin, takes a closure over a runtime struct, and is checked nowhere
  unchecked_today:
    nil_middleware: pwruntime.Compose skips it, so the frame is silently absent where pw panics
    duplicate_name: accepted, where pw panics
    fixed_slots: SlotOperational and SlotAPIDoc accept a frame, which then runs inside the framework frame that shares the number because equal slots keep append order; pw refuses both
  parity_record_is_wrong:
    where: requirement:second-build-feature-parity already_holds says middleware is per backend by design and not a gap, citing decision:backend-specific-middleware
    why_wrong: that decision settles the middleware body, not the registration surface; the body being per transport is what makes a per-transport registration call necessary rather than optional
    action: that entry moves from already_holds to this requirement
  documentation: guides/backend/middlewares.md and its ja twin describe pw only, so a project building with -tags fasthttp finds no answer at all
constraints:
  no_shared_middleware_type: decision:backend-specific-middleware; func(fasthttp.RequestHandler) fasthttp.RequestHandler against func(http.Handler) http.Handler, and an adapter would pull the whole downstream chain onto the other transport
  no_import_time_registry: api:pwfast-package absent_and_why; a chain assembled from arguments must not gain a frame because a package was imported, so the seam has to be named by the application rather than reached from an init
  main_is_already_per_tag: cmd/<app>/main.go and main_fasthttp.go are separate build-tagged files, so a per-transport spelling costs the project nothing it is not already paying
  slot_numbers_are_shared: pwruntime.Slot, aliased by both pw and pwfast; a second number line would make the two builds different applications
  serverless_capture: pw build --target rewrites the entry point's Run into pwfast.Start(ctx, handler, options...) per requirement:serverless-source-entrypoints, so anything carried as an Option reaches the serverless chain with no second mechanism
surface:
  chosen: pwfast.RegisterMiddleware(slot Slot, name string, middleware Middleware)
  why_that_shape: the call is the pw one with one word changed, so the two build-tagged mains differ only in the middleware body -- which is the difference decision:backend-specific-middleware already requires and the only one a project should have to think about
  caller: main, before Run, Start or Middlewares composes; a registration after composition joins nothing, exactly as on the other build
  read_by: Middlewares, so the chain a test assembles through it is the chain Run serves; a registry read only by Run would make the two differ silently, which is the failure the parity is for
  serverless: main runs to its Run call before the generated wrapper captures it, so a registration made there is already in the registry when Start composes
  low_level_seam: RuntimeOptions.Extra is unchanged and stays the plugin's, so authfast and a caller assembling its own frames are untouched
  refusals: nil middleware, duplicate name, and a frame at SlotOperational or SlotAPIDoc, each a panic at registration -- the pw wording and the pw timing, because a project reads one rule for both builds
  not_checked: RuntimeOptions.Extra keeps taking what it is given, which is the pw arrangement too: pw.RegisterExtension does not refuse a fixed slot either
invariant_amended:
  was: api:pwfast-package absent_and_why, no registry here, because a chain assembled from arguments cannot silently gain a frame because something was imported
  still_true_for: an imported capability; a plugin installs frames through RuntimeOptions.Extra and is named by the application, and pwextension.SetupProcess still refuses a net/http frame by name
  now_excepted: the application's own registration from its own main, which is not something an import did
  the_cost_accepted: a registry cannot tell an application's main from a third-party init, so a package could register a frame the application never named; the same cost is already accepted on the other build, and rule coverage rather than the absence of the function is what keeps a plugin out of it
  to_update_on_implementation: api:pwfast-package absent_and_why extension_registry, and requirement:second-build-feature-parity already_holds middleware
alternative_rejected:
  shape: pwfast.WithMiddleware(slot, name, middleware) Option passed to Run and Start
  for: keeps the argument-assembled chain literally true, refuses by returning an error before the port binds, and reaches serverless through the same option capture
  against: a second spelling for one idea, so every guide sentence and every project note forks by build; the refusals arrive as an error where pw panics, so the two builds fail differently for the same mistake; and an application calling pwfast.Middlewares directly would still need the closure
ordering: unchanged from requirement:application-middleware-registration; ascending slot, smallest outermost, equal slots in append order with framework frames first
acceptance:
  - pwfast.RegisterMiddleware takes the pw argument list with this transport's middleware type, and a main pair differs only in the middleware body
  - the frame runs at its slot relative to the framework frames, verified against a real listener rather than a synthesized request value
  - two middlewares at one slot run in registration order
  - a nil middleware, a duplicate name, and a registration at either fixed slot each panic with the pw wording
  - the chain pwfast.Middlewares returns is the chain pwfast.Run serves, registered middleware included
  - a serverless build carries the registered middleware, because main registered before the captured Start composed
  - guides/backend/middlewares.md says where an application middleware goes on each build, and the ja page says the same
non_goals:
  - a portable middleware type or an adapter between the two shapes, refused by decision:backend-specific-middleware
  - an import-time extension registry on this transport
  - per-route middleware, which belongs to the mux on both builds
  - removing or reordering a framework frame; configuration disables frames and numbers only order them
```
