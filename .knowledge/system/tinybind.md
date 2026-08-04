---
id: system:tinybind
type: system
title: TinyBind
---
TinyBind is the generated binding, configuration, response, validation, streaming, and OpenAPI engine wrapped by the Popcorn Wave public APIs.

```yaml
module: github.com/shibukawa/tinybind-go
pin: v0.3.5, moved from v0.2.10 by decision:tinybind-v03-adoption
html_template_baseline: v0.1.15
html_async_baseline: v0.1.20
html_live_baseline: v0.2.8, required by requirement:live-html-rendering; v0.2.7 introduced live boundaries and v0.2.8 answered the first of the integration requests raised against them
route_tree_baseline: v0.2.6
public_wrappers:
  - api:request-binding
  - api:html-response
  - api:api-response
  - api:typed-stream
  - api:problem-response
  - api:runtime-configuration
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
  formatter:
    - the templates/templatefmt package canonicalizes a template source, from v0.3.1, consumed by requirement:template-formatting
    - "Source and SourceAs are pure functions over a byte slice, so an embedder needs no filesystem, no process, and no project"
    - SourceAs names the format explicitly, so the .pw suffixes need no pattern configuration on the editor path; Dir and Identify take the HTMLPattern, SQLPattern, and DynamoPattern options instead
    - a parse failure is carried on the result with the formatted output left nil, so a broken source is never partially rewritten
    - the fmt subcommand is a thin wrapper over the library, with a stdin filter mode selected by -as and a -l listing mode for CI
    - the printer is delegated the same way the parser is: the shared package prints the header and the expressions, each format package prints its own body
    - two spaces per level, a declaration body opening exactly one level, and a soft 100-column width
    - it will not sort, deduplicate, or rewrite one construct into another; SQL keyword case and HTML self-closing syntax are left as authored
  formatter_defects_reported_and_fixed:
    found_in: v0.3.1, formatting every .pw source in this repository, 2026-08-02
    fixed_in: v0.3.2, verified the same way the same day
    non_idempotent_raw_text_escape:
      was: a literal brace run in a script or style body gained one brace pair per formatting pass and never converged
      now: a raw text brace is written back as it stands, because the parser already keeps it as text; only a brace the insertion gate would read as syntax keeps its escape
      note: upstream also drew the pre, textarea, and preserve-whitespace boundary, which are whitespace-preserving but still template text and so still escape
    sql_upsert_split:
      was: "ON CONFLICT(id) DO UPDATE SET was broken across three lines"
      now: the clause absorbs its action keywords and stays on one line
    both: reported upstream rather than worked around here, because a local workaround would have been a second layout implementation
  formatter_idempotence_guard:
    from: v0.3.2
    what: Source and SourceAs format twice and return an error rather than a result that differs between the passes
    where_it_belongs: upstream, which has the AST; it replaces the equivalent check requirement:editor-formatting carried in the extension
    version_floor: an embedder relying on it rather than repeating it must pin v0.3.2 or later
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
  v0_3_1:
    additive_for_generation: templatefmt and the fmt command are new surfaces, so a project that never formats regenerates identically
    cost_of_adopting: every one of the 33 .pw sources in this repository changes, almost all of it the body indent the formatter adds and this repository never wrote
  v0_3_2:
    additive: the formatter fixes and the idempotence guard change no generation output
    unblocks: requirement:template-formatting, whose repository-wide run was waiting on the escape defect
    required_by: requirement:editor-formatting, which stopped carrying its own guard once this one existed
  html_v0_1_15: generated HTML APIs are not source-compatible with earlier direct-writer output
  html_v0_1_19: async parameters and async render entry points are additive, so existing templates and call sites keep compiling after regeneration
  html_v0_1_20: Content.WriteTo narrows to the bare fragment and the module injects no client runtime, so an async caller must supply framing and a runtime it previously inherited
```
