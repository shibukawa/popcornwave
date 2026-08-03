---
id: system:tinybind
type: system
title: TinyBind
---
TinyBind is the generated binding, configuration, response, validation, streaming, and OpenAPI engine wrapped by the Popcorn Wave public APIs.

```yaml
module: github.com/shibukawa/tinybind-go
html_template_baseline: v0.1.15
html_async_baseline: v0.1.20
html_live_baseline: v0.2.8, required by requirement:live-html-rendering; v0.2.7 introduced live boundaries and v0.2.8 answered the first of the integration requests raised against them
html_update_baseline: v0.3.3; v0.3.0 added the htmlupdate package, v0.3.1 handed the asset and every name to the caller per requirement:tinybind-runtime-ownership, v0.3.2 carried head on the action response, and v0.3.3 closed every remaining seam of requirement:tinybind-update-composition-seams and made CSRF module native; adopted by decision:update-runtime-convergence
route_tree_baseline: v0.2.6
current: v0.3.3, which carries the generator crash fix below and, from requirement:module-native-csrf, writes the token into every unsafe form itself
public_wrappers:
  - api:request-binding
  - api:html-response
  - api:api-response
  - api:typed-stream
  - api:problem-response
  - api:runtime-configuration
defects:
  unguarded_position_lookup:
    status: fixed in v0.3.2, which is why this module first moved off v0.2.10
    was: three call sites dereferenced Fset.File(f.Pos()) after guarding f, pkg, and Fset for nil, and that call is the one that returns nil
    sites: generator/plan.go, generator/configbind.go, and generator/dynamobind.go
    fix: each now takes the handle and checks it, which is what generator/configbind_doc.go already did three files away
    symptom_it_removed: a nil pointer dereference in go/token.(*File).Name, taking the calling process down
    trigger: a Go file in a generated directory that does not parse, most often a zero-byte one an editor has created and not yet written into
    mechanism: packages.Load returns a syntax entry for a file it could not parse, that entry reports token.NoPos, and a FileSet lookup of NoPos is nil; measured against golang.org/x/tools with Popcorn Wave out of the picture on 2026-08-02
    downstream_containment_kept: api:cli-generate unparsable_source and its recover stay, because the pre-check names the file and the line where the generator would only name the directory, and the recover bounds every generation panic rather than this one
generator:
  extensible_analysis: requirement:httpbinder-extensible-route-analysis
  openapi:
    - package fragments register during import
    - AssembleOpenAPI returns one deterministic merged JSON document; YAML output was dropped
  configuration:
    - generated definitions register during import
    - ScaffoldTOML and ScaffoldEnv merge every package definition
  html:
    - .pw.html components generate immutable htmlbind.Fragment values
    - components with an unnamed slot also generate htmlbind.Wrapper binders
    - api:render-html-chain composes wrappers around a leaf
    - "`external async` declarations bind Go functions returning a value and an error"
    - "`async T` parameters and record fields become htmlbind.Pending[T], surfaced by api:async-html-value"
    - await, fallback, and recover clauses compile to boundaries described by api:html-boundary-protocol
    - the generated plan carries a constant HasAwaitBlock flag used by decision:automatic-async-render-selection
    - the async render path emits placeholders and bare fragments; completion framing and the client runtime belong to the framework
    - "`external live` declarations bind Go functions of the shape func(ctx, args...) iter.Seq2[T, error], with the context mandatory rather than optional"
    - a live binding sits in an ordinary await clause, so there is no second clause keyword and one clause may mix a live binding with a settle-once one
    - "RenderChain renders a live boundary from its first delivery; RenderChainAsync commits the first delivery and unsubscribes; RenderChainLive keeps delivering and does not end"
    - the entries that must answer bound how long a boundary may show nothing, and running out leaves the fallback rather than rendering recover
    - HasLiveBlock reports whether a chain keeps changing after the document ends, a subset of HasAwaitBlock
    - Content.AppendJSON writes one delivery as a JSON record, escaped for a script context as well as a JSON one
    - boundary ids became positional, so a nested boundary is tb-1-1 and the same chain executed again produces the same ids; api:html-boundary-protocol carries what that changes here
    - "the live entry does not enforce that the body writer is discarded, so passing a real writer produces an endless document response"
    - a live failure reaches the error reporter after the delivery lock is released, from v0.2.8; before that a blocking reporter held the clause's goroutines
    - nothing states which boundary is live, so requirement:live-boundary-liveness-signal is still answered by the framework's own bookkeeping
    - a live render executes the whole composed chain, so requirement:live-mode-plan-slice is still paid per reconnect
  html_update:
    - the htmlupdate package holds every net/http concern of partial updates, so htmlbind stays free of it and generated plans keep working on TinyGo and WebAssembly targets
    - every layout and page of a rendered chain is an update boundary automatically; an ordinary component call is not, and the document shell never is
    - a boundary must render exactly one root element, and a component that cannot is simply not a boundary rather than a generation error
    - two keyed digests per boundary, the frame validator over its own bytes excluding nested boundaries and the input validator over its declared parameters; the frame one is the authority for omitting a boundary
    - a delta skips transmission and never execution, so only a component opting into output caching skips its own render
    - Options carries the validator key, the header prefix, the path prefix, the build identity, and the manifest size cap, and pw wraps it as api:html-update-options
    - Negotiate resolves anything unrecognized to a complete document, which is what lets a live token share the header per decision:update-runtime-convergence
    - Mount installs the runtime asset and the redraw endpoint under one path prefix; pw serves its own merged asset and takes only the redraw route
    - a reloadable modifier on a component declaration generates a typed query decoder and a registration value, consumed by requirement:reloadable-component-endpoint
    - Registry.Register panics on a repeated kind, because the kind covers name, parameters, and markup but not the package
    - WantsUpdate, WriteUpdate, WriteUpdateStatus, and WriteNavigate are the action-response surface requirement:action-response-update branches on
    - the generator gained a data attribute prefix option naming the boundary attributes, which pw sets to its own brand
    - from v0.3.1 a render option names the async placeholder element and the boundary identifiers from that same prefix, so one document no longer holds two spellings
    - from v0.3.1 the browser runtime source, its asset form, and its naming configuration are exported, and serving it is switchable, so a framework composes it into its own asset instead of copying it
    - the runtime is a factory reading its attribute prefix, header namespace, endpoint prefix, and installed name from that configuration; only the protocol version stays compiled in, and an empty installed name installs no global
    - the author-written preserve and ignore markers follow the configured prefix, so no application template carries the module's name
    - Mount takes a one-method router interface satisfied by api:serve-mux, registration returns an error beside a must-variant, and an options validator reports every unusable option at once
    - a failure callback receives every refused redraw with a kind, status, message, cause, and the component and instance it named, so a refusal reaches api:error-renderer and requirement:modern-observability
    - the redraw response carries a keyed ETag with a private, no-cache policy, so an unchanged region answers 304; the policy, the query bound, and the stream media type are all options
    - builtin element registration is unimplemented at v0.3.0 and lands in v0.3.3, so a framework-supplied element had no registration seam until then
    - a synchronous external declaring a leading context.Context receives the render context, and one returning html lowers to a slot, which was the interim shape planned for a framework CSRF element
    - style and script blocks extract to content-hashed files under a configured public directory, unused until requirement:component-asset-extraction sets the options
  route_tree:
    - the routetree package discovers a directory tree and writes the registrations, which is the opposite direction from the registered-router analysis above
    - one run covers one tree; requirement:discovered-page-routing wraps it and flow:page-route-generation drives it
    - reserved file names, generated file names, called symbols, five named blocks, and three whole-file templates are all configurable, which is the seam decision:page-render-binding uses
    - the render block carries every in-scope identifier by name, including a chain that is nil for a page with no ancestor layout, so a framework reshapes the call instead of renaming it
    - the router type and its constructor are their own symbols, separate from the package supplying Request, and an empty constructor name omits the constructor
    - the composer entry takes a configurable writer type and an optional request parameter, with imports derived from the signature
    - a query input declared with a trailing question mark binds a pointer, so an absent value is distinguishable from a zero one
    - the discovered tree reports its package list, which is what lets a framework run request binding over route packages
    - discovery skips files carrying tinybind's own generated header, plus any header prefix the run registers, and deliberately does not skip another tool's generated code
    - the emitted header is settable and pairs with an accessor returning the prefix to register, so a branded output cannot drift from what discovery recognizes
    - the failure entry a generated handler writes through is a symbol, so a framework naming it something else needs no template override
    - a package with no bindable model reports a wrapped sentinel error, so a loop over many packages can skip it with errors.Is
    - an action resolver answers a server-action name the current tree does not hold, which is what would let a flat template use one
    - htmlbind.Signatures and htmlbind.ActionRefs expose component parameter types and template action references for a framework generating around a template
  sql:
    - decision:tinybind-sql-runtime owns statement plans and shared execution
    - declared sql.exec, sql.one, sql.optional, or sql.many selects Exec or Query
    - incompatible SQL result contracts fail generation
    - a SQL dialect option selects the target engine, required from v0.2.2 with no default
    - the dialect carries the placeholder style, dollar for postgresql and question for mysql and sqlite
    - postgresql, mysql, and sqlite from v0.2.3, which covers every decision:server-sql-support-tier first-class engine
  dynamo:
    - the dynamobind runtime package and a generator mode, from v0.2.8, consumed by requirement:dynamodb-generation
    - a dynamo struct tag names the attribute and its options, and an unknown option is a generation error rather than a silently ignored string
    - each tagged type yields EncodeItem, DecodeItem, ItemKey, and a table definition constructor, with compile-time assertions against the runtime interfaces
    - codec emission is usage-directed, so a type gets only the halves a discovered call needs; the key builder and the table constructor are emitted whenever a partitionkey tag exists
    - the table constructor carries the name, the partition key, and the optional sort key; billing mode, capacity, and secondary indexes are left zero
    - dispatch is static, so no registry or init entry is emitted and an unused type links nothing
    - the operation helpers take a driver client argument and pass driver errors, retries, and page boundaries through untouched, which is why decision:dynamodb-no-runtime-abstraction wraps none of it
    - from v0.2.9 a query declaration file generates one named function per access pattern, consumed by requirement:dynamodb-typed-queries
    - from v0.2.10 the client is carried in the context and set with a client setter, so no entry of the package takes one
    - the same setter takes an optional table resolver function, run inside every runtime entry, which is the seam rule:dynamodb-table-naming installs
    - a declaration carries a required table clause, so a generated query names neither a client nor a table
    - a missing client is a named error rather than a panic, so an entry reached without the middleware fails as an ordinary error
    - the declaration suffix is configurable through DynamoTemplatePattern and the output file name through DynamoQueryName, so Popcorn Wave brands both without renaming anything after generation
    - a declared query's attribute names are checked against the tags, and every attribute is aliased unconditionally so no reserved word reaches an expression literally
    - the string key-condition form remains as an unchecked escape hatch
    - table definition emission is suppressible as the named feature item-table, and the whole mode as item-codec
    - single-table design is a stated non-goal, so one struct owns one table
    - a version tag for optimistic locking and a ttl tag are proposed, the latter blocked on the driver
    - no update or condition expression is generated, and secondary index tags are deferred
    - no generation option selects a framework resolver, unlike the SQL executor resolver, because resolution moved into the runtime and left no generated call site to redirect
constraints:
  - a route tree directory name must be a legal Go import path element, per rule:page-directory-naming
  - generator executes with host Go
  - generated mapping path avoids runtime field reflection
  - route discovery analyzes same-package registrations recognized by versioned adapters
  - normal handwritten application code does not import TinyBind
compatibility:
  route_tree_v0_2_4: the routetree package and server-action lowering are additive, so the pin moves from v0.2.3 without touching an existing handler, template, or query
  route_tree_v0_2_5:
    additive: every new seam is additive and the default output is byte-identical, so the pin moves again without regenerating differently
    behavior_change: discovery now skips tinybind's own generated files, which removes registrations a run could previously read back out of a generated registry
  route_tree_v0_2_6:
    additive: the header and the failure selector become configurable with defaults matching what v0.2.5 emitted
    resolves_for_pw: api:cli-generate writes its own generated header, which the v0.2.5 filter could not recognize; registering the prefix is now the supported answer, so page tree output keeps the Popcorn Wave brand
  sql_v0_2_2: the SQL dialect became a required generation input, so a run that discovers a .pw.sql without one is a configuration error rather than a silent postgresql assumption
  sql_v0_2_3: the sqlite dialect is additive and emits the question placeholders sqlite already generated through mysql, so naming it changes no generated output
  dynamo_v0_2_8:
    additive: dynamobind is a new package and a new generator mode, so nothing an existing project generates changes
    module_graph: the module now requires system:tinygodriver v1.1.3, because a runtime package imports the DynamoDB client rather than only an example doing so
  dynamo_v0_2_9:
    additive: the query declaration is a new source kind and a new output file, so a project generating only codecs regenerates identically
    answers: the downstream Popcorn Wave request, whose allocation decision:dynamodb-framework-scope records
    closes: the read-path drift requirement:dynamodb-generation could not close on its own
  dynamo_v0_2_10:
    breaking: every runtime entry lost its client parameter and a declaration gained a required table clause, so v0.2.9 call sites and declarations both need editing
    scope_for_pw: nothing was released against v0.2.9, so the change costs an edit to these concepts rather than to a project
    size: about 37 KB on a TinyGo wasip1 build, from the context value and the assertion reading it back
    answers: the second downstream request, and answers it by removing the seam rather than adding one
  v0_3_2:
    taken_for: the unguarded position lookup above, which crashed api:cli-generate on a file an editor had created and not yet written into
    arrives_with: the boundary emission requirement:navigation-delta-rendering consumes, whose activation is opt-in per component except for generated route layouts, which take it automatically
    effect_on_pw: a concept:page-tree component now emits a boundary marker attribute and one update-manifest entry; the rendered document gains an attribute and loses nothing
    measured: one page tree fixture regenerated, and the rest of the suite passed unchanged, so no Popcorn Wave source needed editing
    superseded_by: v0.3.3 and the adoption decision:update-runtime-convergence records, so the markers are no longer inert; requirement:module-native-csrf is the half taken first
  html_v0_1_15: generated HTML APIs are not source-compatible with earlier direct-writer output
  html_v0_1_19: async parameters and async render entry points are additive, so existing templates and call sites keep compiling after regeneration
  html_v0_1_20: Content.WriteTo narrows to the bare fragment and the module injects no client runtime, so an async caller must supply framing and a runtime it previously inherited
  html_v0_3_0:
    additive_on_the_wire: a project that never sends the render header renders and serves exactly as it did, so the pin moves without regenerating differently
    generated_output: boundary attributes and validators appear on layout and page roots, which changes generated markup for every page even when no update is enabled
    duplicated_here: the module shipped a browser runtime, a header namespace, and an endpoint prefix that overlap what this framework already owns; decision:update-runtime-convergence decides what happens to each
    not_a_drop_in: the shipped runtime was built for the upstream names, so adopting the transport without adopting the names would have cost an adapted copy of its source
    superseded_by: v0.3.1, so this version is never the one to pin
  html_v0_3_1:
    answers: requirement:tinybind-runtime-ownership in full, which is what makes the transport adoptable without a copy
    generated_go: unchanged, so the pin moves without regenerating differently
    breaking_for_a_direct_user:
      - the preserve and ignore attributes default to the module's short prefix rather than its full name
      - the query bound and stream media type constants were renamed as defaults, having become options
      - registration returns an error rather than panicking
    breaking_here: none, because nothing was released against v0.3.0
    upstream_correction: the embedded runtime was an interim shape its own rollout requirement had recorded, whose exit was never scheduled, rather than a reversed boundary; the effect downstream was the same and requirement:tinybind-runtime-ownership carries the correction
    still_interim_upstream: the module serves an asset by default and retires that only when its own runtime bootstrap selects and injects one; this framework declares caller ownership, so the default never applies here
  html_v0_3_2:
    additive: an action response gained a head field and the rest is documentation, so nothing generated or served changes for a project that does not use it
    action_head: each written region's own contributions are collected and deduplicated across the set; the browser already installed a delta's head before applying operations, so only the server was never filling it
    live_transport_confirmed: the module's document render settles a live boundary in place and finishes the response, and a second connection carries deliveries, which is this framework's own shape rather than a divergence
    live_token_still_absent: no live token is parsed on either side, and the shipped upstream runtime sends the navigation token for both the first connection and every reconnect; filed as a must-priority requirement recommending a live token, which is this framework's existing choice
    still_open: the redraw response carries no head, the slot-carried fragment head defect, and what a fragment response owes a caller it cannot deliver to, each filed as its own requirement rather than settled
  html_v0_3_3:
    answers: every remaining item of requirement:tinybind-update-composition-seams, and moves CSRF into the module
    live_mode: a live token of its own with its own negotiated mode, so subscriptions stay open only in that mode; termination reasons name final, live-pending, failed, done, and retry, a retry may carry a server-side delay hint, the head record carries the build, and a cancelled context closes as retry rather than done
    live_handoff: a response header, a delta body field, and a stream terminator each say whether a live connection is expected, and none appears when the page has no live boundary
    adopted_from_here: the done-versus-retry distinction, the build on the opening record, and resetting the attempt count on a healthy close were this framework's shipped behaviour, offered as input and taken
    live_validators: a delivery carries none and the opening delta does, which answers the question that item left open
    live_defect_fixed_upstream: the live entry had set subscriptions unconditionally, so an ordinary navigation delta on a live route never terminated
    redraw_head: the registry reports the head and assets of every published component for the shell to install once, and a redraw that contributes head announces it on a response header; the body stays a bare subtree
    asset_set: an asset value on the plan with fragment, wrapper, and merge accessors, readable before rendering and folded through slots
    slot_head: the plan reaches fragments carried in parameter structs, so head, sources, assets, and capability flags all fold; a project declaring no html parameter regenerates identically
    vary_axes: a composition reports the request properties its output varies on, so a response can set an honest Vary header rather than guessing
    builtin_elements: a framework registers hyphenated elements that lower to plan steps at generation time, with the value never entering template scope
    csrf: consumed by requirement:module-native-csrf
    protocol_version: deliberately left at 1, because nothing has shipped under it and spending a bump would cost the first real deployment one wasted fallback
  breaking_v0_3_3:
    hyphenated_elements: the namespace is closed, so a project writing Web Components must declare them; requirement:custom-element-registration carries what this framework owes projects
    cached_unsafe_form: a component holding an unsafe form can no longer be output-cached, which policy:csrf-protection records
    scaffolds_unaffected: no template this framework scaffolds writes a hyphenated element or caches a form, so a scaffolded project regenerates identically
  not_built_upstream_v0_3_3:
    - the opaque builtin element shape, declined because the trust assertion would move into framework code
    - a builtin element inside a head declaration, so head placement means only that the body position is refused
    - the embedded asset byte table and the caller-supplied URL function, which is what requirement:component-asset-extraction needs for a TinyGo target
    - a server-side lifetime bound calling the retry seam, left to the caller
    - the redraw head bound as an option, since registration cannot reach the options value
  known_flake_upstream: a superseded live delivery can report a stale value into a reused placeholder, reproduced on the baseline and predating v0.3.3, so a cancellation-ordering race rather than a regression
  other_v0_3_1_features:
    template_formatters: formatters for the html, sql, and dynamo template languages, unevaluated here and a candidate for api:cli-generate or a project check
    asset_transform_hooks: build-time rewriting of referenced assets through registered hooks, which is the seam requirement:component-asset-extraction would build on if that work is taken up
```
