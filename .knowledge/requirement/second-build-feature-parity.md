---
id: requirement:second-build-feature-parity
type: requirement
title: Feature Parity For The fasthttp Second Build
---
Every framework capability an application can reach on net/http must be reachable on the fasthttp build. Nothing is out of scope by intent; what is listed below is unfinished, not declined.

```yaml
status: proposed
serves: requirement:alternate-http-backend-readiness
measured: 2026-08-11 on the merge of the fasthttp branch into main
method:
  containment: go list -tags fasthttp -deps <pkg> must not contain github.com/shibukawa/popcornwave/pw
  surface: go doc -all ./pw vs ./pwfast, each missing name then searched across every shared leaf
  call_registry: pw exported funcs taking http.ResponseWriter or *http.Request, diffed against the patterns in internal/pwgen/options.go
already_holds:
  call_registry: all 23 transport-taking pw funcs are registered, so no application call shape is refused for want of a pattern
  page_runtime: pwpage has pwfastpage
  middleware: decision:backend-specific-middleware already records that pw.RegisterMiddleware is per backend by design, not a gap
class_a_reachable_under_another_name:
  defect: not missing capability; the only spelling published is the pw one, so app code shared by both builds names a package the second build does not link
  rule: rule:shared-code-names-a-leaf
  mapping:
    pw.RegisterConfig: pwconfig.Register
    pw.ParseConfig: pwconfig.Parse
    pw.SetConfigLoadOptions: pwconfig.SetLoadOptions
    pw.RegisterExtension: pwextension.Register
    pw.RegisterSessionBackend: pwsession.RegisterBackend
    pw.OpenSessionBackend: pwsession.OpenBackend
    pw.SessionBackends: pwsession.Backends
    pw.SessionManager: pwsession.Manager
    pw.SessionRegistry: pwsession.NewRegistry
    pw.SessionKeyring: pwsession.Keyring
    pw.SessionCookiePolicy: pwsession.CookiePolicy
    pw.SessionPrune: pwsession.Prune
    pw.RegisterSessionStore: pwsession.RegisterStore
    pw.RegisterRateLimitStore: pwratelimit.RegisterStore
    pw.RateLimitStores: pwratelimit.Stores
    pw.StartSpan: contrib/otel/trace.Start
    pw.StartSpanKind: contrib/otel/trace.Start with WithSpanKind
    pw.NewSignal: pwruntime.NewSignal, fixed 2026-08-11
  found_after_the_measurement:
    pw_log_attributes:
      names: Err, String, Int, Int64, Float64, Bool, Duration, WithLogAttributes, and the Level constants
      state: fixed 2026-08-11 in pwfast/log.go
      why_the_surface_diff_missed_it: pwfast.Logger existed, so the logging surface looked present; the diff compares names one at a time and nothing asked whether the ones that crossed were usable without the ones that did not
      lesson_for_the_method: a partially crossed surface passes a name-by-name diff and fails to compile, which is the failure this requirement exists to find
  open_question: whether a leaf spelling is enough or the pw names should also resolve, since an application reads pw docs and finds no note that the name it copied is net/http-only
class_b_no_implementation_anywhere:
  openapi_document_and_ui:
    names: AssembleOpenAPI, SetOpenAPIInfo, ScalarUI, SwaggerUI
    partial: pw.OpenAPIJSON is a registered transport call, so the JSON endpoint already crosses; assembly, the info block and both doc UIs do not
  package_assets:
    names: RegisterPackage, Packages, PackageAssetURL
  lifecycle_headers:
    names: LifecycleHeaders
class_c_packages_that_block_whole_examples:
  sessionstore:
    packages: sessionstore and its dynamo, firestore, mysql, postgres, redis, sqlite backends
    cause: they name pw session symbols only; every symbol exists in pwsession or sessionconfig
    cost: mechanical retarget, same shape as the signal fix
    blocks: examples/oidclogin and examples/passkeylogin cannot declare fasthttp = true
  testutil:
    cause: names pw.PrepareTestRuntime, pw.SnapshotTestConfigs, pw.Transaction, pw.DB and the config binder aliases
    partial: fasttestutil exists but holds only exchange.go
class_d_absent_by_deferral:
  websocket:
    state: done 2026-08-11; examples/websocket_chat declares fasthttp = true, compiles under both tags, and names no pw in the tagged dependency graph
    requirement: requirement:typed-websocket, which replaced requirement:contrib-websocket once the capability turned out to be upstream already
    library_exists: system:tinybind-websocket publishes an entry on both transports, so the gap is this framework's surface rather than a dependency
    parity_is_the_forcing_reason: the entry has to be spelled pw.WebSocket and pwfast.WebSocket for the same rewrite this requirement measures
class_e_toolchain: decision:tinygo-fasthttp-needs-nozstd
acceptance:
  - an example declaring fasthttp = true exists for every capability listed above
  - go list -tags fasthttp -deps of every such example contains no pw
  - the pw doc for a class_a name says which package a shared file must call instead
```
