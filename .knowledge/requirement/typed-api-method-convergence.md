---
id: requirement:typed-api-method-convergence
type: requirement
title: Typed API Method Convergence
---
Every typed operation a project writes as a package function only because a Go method cannot take type parameters becomes a method once the language allows, and this is where that intent and its upstream requests are held.

```yaml
status: the available-today half shipped upstream in v0.5.9; the rest is blocked on the language
constraint: a Go method may not declare its own type parameters, so an operation needing one is a package function whatever the design prefers
expected: hoped for Go 1.27, roughly 2027-02 given 1.26 in 2026-08; not a committed language feature, so the release notes decide and nothing here breaks if it slips
why_it_is_held_here:
  the_surface_is_this_framework_s: api:dynamo-package and api:firestore-package wrap no operation, per decision:dynamodb-no-runtime-abstraction, so an upstream package function is literally what an application author writes
  the_flow: this catalog carries the intent, the priority, and the shape; the upstream change is requested from here when the work is taken, the way system:tinybind already records input offered and taken
migration_shape:
  method_becomes_the_body: the existing package function stays as a deprecated wrapper calling it
  additive: a project moves call site by call site and no compiler error forces any of it
  nothing_stored_moves: each of these is a call shape, so no generated artifact, stored entry, or wire format changes
sites:
  firestore_transaction:
    priority: first, and the only one whose value is more than tidiness
    now: writes are methods on the transaction while typed reads are package functions, so one transaction is written two ways in adjacent lines
    becomes: Load, LoadAll, and QueryPage on the transaction value
    why_it_matters: the transaction boundary stops being a parameter and becomes the receiver, so the API states what the call is inside
    already_recorded_upstream: the comment on the transactional read names the language as the reason it is a function
    survives_the_move: the same comment separately refuses a context-carried handle, because one call site would then mean two things depending on which context reached it; that reason is untouched, so the operation is reached through the transaction value either way
    owner: system:tinybind firestorebind
  handle_entries:
    priority: second
    now: a context-resolving form and an explicit-handle form stand side by side, the latter suffixed On, which api:dynamo-package and api:firestore-package both expose as the supported call
    becomes: the On form is a method on the concrete handle; the context form stays as it is
    extra_win: an operation whose type is inferable from its argument loses its type argument entirely — storing, storing many, and removing all read as plain calls
    dynamo: LoadOn, LoadAllOn, StoreOn, StoreAllOn, StoreReturningOn, RemoveOn, RemoveReturningOn, UpdateOn, QueryPageOn, QueryOn, ScanPageOn, ScanOn
    firestore: LoadOn, LoadAllOn, StoreOn, StoreAllOn, InsertOn, InsertAllOn, UpdateOn, RemoveOn, RemoveAllOn, QueryPageOn, QueryOn
    owner: system:tinybind dynamobind and firestorebind
  html_builder:
    priority: third, and the least visible
    partly_done: Require moved to the builder in system:tinybind v0.5.9 with the function kept as a deprecated wrapper, since it carried no extra type parameter; For, ForCtx, Await, Live, and Provide are the ones still waiting
    now: ordinary builder operations are methods while the four carrying an extra type parameter are package functions
    which: the loop and its context form, the await boundary, the live boundary, and the provider entry — ForCtx belongs here with For and was missed the first time this list was written
    becomes: methods on the builder, so generated code stops mixing two spellings in one plan
    smaller_because: this is generated output rather than authored code, so no application reads it
    owner: system:tinybind htmlbind
  test_configuration:
    priority: with the memo work, since both are this framework's own
    now: an isolated test configuration is a struct whose three typed operations are package functions taking it
    which: Get, Set, and Update of testutil
    becomes: methods on the configuration value, so a test reads config.Get rather than naming the package twice per line
    inferable_but_still_blocked: two of the three infer their type from an argument, and a method still may not declare a type parameter even when nothing has to be written, so all three wait together
  session_registry:
    priority: with the memo work
    now: Register takes the registry as its first argument because it carries a typed codec
    becomes: a method on the registry, with the type inferred from the codec
  json_parser:
    priority: with the html builder, since the caller is generated code
    now: the parser is a struct with a dozen methods, and the two operations parameterized on the decoded element are package functions taking it
    which: ParseSlice and ParseMap of jsonbind
    same_shape_as: the firestore transaction, one type written two ways in adjacent lines
    owner: system:tinybind jsonbind
  sql_builder:
    priority: last, being one function
    now: the statement builder has methods, and appending a typed value list is a package function taking it
    which: AppendValues of sqlbind
    owner: system:tinybind sqlbind
  memo:
    priority: fourth by value, first by readiness
    what: the typed data cache operations move onto the store handle, per decision:memo-store-handle
    already_prepared: the handle is introduced before the methods can exist precisely so this move edits no call site
    owner: this framework, which is why it is the one site not requested from anyone
requests_upstream: the firestore transaction, the handle entries, the html builder, the json parser, and the sql builder, each being system:tinybind's code and none of them reachable from here
built_here: the memo store, the test configuration, and the session registry
ruled_out_deliberately:
  an_interface_receiver: sqlbind ScanRows takes the row cursor, which is an interface, so the package cannot give it a method however the language changes; it stays a function
  the_context_forms: every ctx-resolving entry — the dynamo and firestore non-On forms, the configuration accessors, the session slot lookup — has no receiver by design and is the half that stays, per api:dynamo-package
  constructors: a store, jar, or codec constructor has nothing to receive on
  foreign_receivers: the response and websocket writers take net/http or fasthttp types, which no package here may extend
  process_registries: the configuration seed, bound, swap, and value entries address process state rather than a value
possible_without_the_language:
  what: three htmlbind entries introduced no type parameter beyond the one their receiver already carried, so each was a legal method in today's Go
  which: Require on the builder, Bind and BindWrapper on the plan
  shipped: system:tinybind v0.5.9, with each old function kept as a deprecated wrapper
  cost_here: none; the only call sites in this framework are in tests, which still compile
  why_separating_them_paid: bundling them into the blocked list would have hidden that they were available, and they shipped in the same release as the blocking ask rather than waiting on a language change
acceptance:
  - a project that migrates nothing keeps compiling, because every replaced function survives as a wrapper
  - the transaction boundary appears as a receiver rather than as an argument
  - no generated artifact, stored cache entry, or wire format changes in any of the four
```
