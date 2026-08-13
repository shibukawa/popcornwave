---
id: decision:component-handler-namespace
type: decision
title: Component Handler Namespace
---
Resolve a template-named handler against what the instance's `setup` returned, and nothing else, and ask system:tinybind to read that at generation so the name is checked rather than discovered in a browser.

```yaml
source: requirement:component-script-event-binding, and the user's two proposals of 2026-08-11 with the direction that system:tinybind should do it automatically
review_gate: proposed
the_question: which namespace a name in the markup resolves in, which decides what a handler can see and whether anything can check it
why_it_is_not_a_spelling_question: decision:event-binding-attribute-spelling settles what the author writes; this settles what the name means, and the two answers are independent
candidates:
  module_exports:
    shape: the runtime reads the declaration's module namespace, which boundary.js already holds after the dynamic import
    scope: module top level, evaluated once per URL for the life of the document
    fatal_flaw: the guide this framework already publishes warns that module scope is shared by every instance forever, so a component rendered twenty times in a for block binds one function to twenty elements and that function cannot see the state its own instance's setup built
    what_it_reduces_a_handler_to: whatever it can re-derive from event.currentTarget, which is the awkwardness the per-instance mount exists to remove
    was: what an earlier revision of requirement:component-script-event-binding specified, and it was wrong for this reason
  returned_namespace:
    shape: 'setup returns an object, in the manner of a Vue composition setup: return { increment }'
    scope: the setup call, so a handler closes over that instance's own locals with no ceremony
    fits_what_already_ships: setup is already called once per instance at pwbrowser/boundary.js:721, so the namespace is a value the runtime is already holding rather than a new channel
    second_gain: the exposed surface becomes intentional, where module exports publish every helper an author exported for a test or for a sibling module to import; this is the inverse of the cost api:page-action-endpoint accepts when it says exported is published
    borrowed_shape_not_borrowed_semantics: Vue also exposes state through that object and re-renders from it, and nothing here does; interpolation is server-side Go, so this object is a handler namespace and an author arriving from Vue will expect more than it gives
  parsed_top_level:
    shape: generation reads the block, finds a top-level function declaration, and the template names it
    scope: module top level, so it has the same sharing property as module_exports and the same blindness to instance state
    unique_gain: it is the only candidate a compiler can check, because a top-level function declaration is a thing a parser can enumerate
    unique_cost: it needs a real JavaScript tokenizer, since deciding that a function keyword is at top level means skipping strings, template literals, comments, regular expression literals, and nested scopes; a pattern match over the text gets this wrong on ordinary code
chosen:
  only: the returned namespace, because a component script's whole model is per-instance and a handler that cannot reach its instance's state is the case this feature exists to serve
  no_fallback: decided 2026-08-11; a top-level declaration is still where a shared or unit-tested function lives, and the author returns it by name, so the fallback would have saved a return and charged a lookup rule for it
  one_place: a name the returned namespace does not hold fails, with no second namespace to adjudicate against
  the_bag_reaches_every_handler: a consequence worth naming, since every handler is now defined in or returned from setup and therefore sees what setup was handed
automatic_means_checked:
  the_ask: the user's direction is that system:tinybind does this rather than each framework, which makes generation-time resolution the point rather than a refinement
  consequence: the parser the parsed_top_level candidate needs is a prerequisite for the chosen design too, not an alternative to it
  what_a_parser_buys_for_the_returned_namespace: a setup whose last statement returns an object literal has statically readable keys, which is the idiomatic form; a setup returning a variable assembled conditionally does not
  therefore: checking is total for the shapes an author actually writes and partial in general, reported as unverifiable rather than passing silently, which is the position the field checking of system:tinybind already takes for a form control it cannot attribute
  who_reads_the_block: this framework, per decision:script-block-parsing-ownership, so the module keeps its position that it reads no JavaScript and the ask becomes an option on a seam it already has
  reconsider_marker_already_exists: system:tinybind records that the export set becomes readable once an authored-language transform puts a real parser in the pipeline, so this is that moment arriving from a second direction
the_return_slot_is_freed_rather_than_shared:
  today: setup's return value is the teardown, read at pwbrowser/boundary.js:721 and taught in the component scripts guide in both languages with its own rationale
  chosen: 'teardown becomes something setup is handed rather than something it returns: setup({ el, teardown }) with the argument destructured, decided with the user 2026-08-11'
  effect: the return slot has exactly one meaning, so the namespace needs no typeof union, no reserved key, and no diagnostic for a handler that collides with one
  registration_beats_a_return_on_its_own_merits:
    - it can be called more than once, so two subscriptions do not have to be closed over into one function
    - it can be called conditionally and from inside a helper, where a return value has to be threaded back out
    - a fourth capability later is one more key rather than a third positional argument
  destructuring_is_why_the_bag_works: an author names only what they use, so a component wanting neither signals nor cleanup writes setup({ el }) and reads nothing it does not need
  release_order_is_the_documented_one: signal registrations release first and teardowns run after in reverse of registration, which is what the guide already states about the scope running before the returned teardown
  late_registration: a teardown registered after setup returned, from inside a handler, is honored, since the bag outlives the call and release has not happened yet
  vue_precedent: onUnmounted is a registration rather than a return there too, and the reason is the same one
migration:
  done: 2026-08-12
  breaking: yes, and deliberately so
  no_compat_shim_is_possible: fn.length is 1 for setup(el) and 1 for setup({ teardown }) alike, so no arity or shape test tells the two apart and nothing can support both
  fails_loudly: an unmigrated body receives an object where it expects an element, so el.querySelector throws at mount and is reported, rather than binding nothing and looking healthy
  surface_in_this_repository: two authored templates, internal/pagesfixture and examples/live_render, plus their regenerated assets and eight documentation pages
  owner: this framework alone; system:tinybind states that setup, its argument, and the teardown convention are the caller's, so unlike the parser this needs no upstream agreement
  timing_argument: the surface is two files today and grows with every project that adopts component scripts, and this feature already forces a version boundary for the parser, so the break rides one that is happening anyway
  what_it_does_not_touch: the marker, the asset link, the mount and release points, and the wire, none of which name the signature
async_is_awaited:
  decided: with the user 2026-08-11, on the grounds that refusing a promise is the harder position to hold now
  setup: a thenable return is awaited and binding happens after it resolves, which widens the inert window an author already controls
  rejection: mounts nothing and is reported, the same path a throwing setup already takes at pwbrowser/boundary.js:726
  no_double_mount: the element is claimed before the import resolves, so a scan during the await does not start it twice, which is behaviour that already ships
  ordering_is_kept_along_the_ancestor_chain:
    today: mountScopesIn walks in document order so an ancestor's setup runs before a descendant's, which the component scripts guide states as a property
    asked: 2026-08-11, that a setup should also start only after the previous one settled, on the grounds that nothing here is slow
    the_principle_is_taken: a descendant waits for its ancestors to settle, so what the guide promises stays true rather than weakening to started-in-order
    the_scope_is_narrowed: siblings do not wait for each other, because the documented guarantee is about ancestors and descendants and says nothing about two components in unrelated parts of a page
    why_not_fully_serial: an await that never settles is a real shape — customElements.whenDefined for an element nothing defines, or a dynamic import failing slowly on a bad network — and under one queue it stalls every remaining component on the page, silently and with no diagnostic
    what_the_narrowing_costs_that_case: the hang stops at its own subtree, whose descendants may legitimately depend on it, so the blast radius matches the dependency
    identical_today: every shipped setup is synchronous, so both readings produce the same order at the same cost; they diverge exactly when someone writes an async setup, which is also when the hang becomes possible
    machinery_exists: mounted is already keyed by element and can hold the pending promise, and the nearest ancestor marker is the parentElement closest lookup the runtime already performs at pwbrowser/update.js:565
reaching_the_component_s_parameters:
  asked: 2026-08-11, whether a block writes {id} to read a parameter
  answer_today: it cannot, and not by omission
  why_interpolation_is_impossible_here:
    verbatim: the script block of system:tinybind v0.5.5 reads the block as authored text to its closing tag, so a brace in it is the character rather than an insertion; that is an acceptance condition of the feature rather than a limitation of it
    one_file_for_every_render: the block is extracted to a content-hashed file at generation, and two components whose bytes match share one file, so there is no per-render substitution to make
    it_is_what_makes_it_cacheable: a file that varied per render could be neither content-hashed nor immutably cached, which is the whole delivery model of requirement:component-asset-extraction
  what_works_with_no_new_machinery: the author renders the value into the markup and setup reads it off the element, which is what el is for
  nothing_carries_them_today: the scope marker system:tinybind writes holds a declaration identity, and requirement:reloadable-component-endpoint writes an instance id and a kind, so no parameter value reaches the DOM anywhere
  the_props_channel:
    proposed: by the user 2026-08-11; generation reads setup's own parameter pattern, matches the names against the component's declared parameters, and emits just those as JSON on the component root for the runtime to hand back
    supersedes: an earlier answer here recommending a separate declaration, which this improves on rather than contradicts
    the_destructuring_is_the_declaration: an author names what crosses by writing it in the code that uses it, so there is no second list to keep in step and nothing can drift
    rejected_automatic: serializing every parameter of every component declaring a block puts values in the DOM the markup deliberately did not show, which is the projection hazard rule:client-event-authoring states for signal payloads arriving through a second channel; naming the subset removes it, because nothing crosses that the author did not write down
    the_type_rule_is_the_json_one: corrected 2026-08-12 against v0.5.8; a record and an array of accepted types are accepted recursively, and html and an unsettled value are refused, per decision:component-parameter-disclosure which records why the reloadable rule this once named is the wrong one
    still_an_error_rather_than_an_omission: on the grounds the reloadable diagnostics already give, that the author asked for it by naming the parameter in code that uses it
    checking_improves_rather_than_costs: the parser this feature already needs reads the pattern, so a name that is neither a capability nor a declared parameter fails generation naming both, which is what server-action already does for a handler that does not exist
    json_rather_than_one_attribute_each: a dataset read is text whatever the Go type was, and a JSON value keeps a number a number, which is the one thing the read-it-off-the-element pattern of rule:template-behavior-attributes cannot do
    escaping: an attribute insertion under the ordinary escaper, not a script context, so nothing new is trusted
    the_block_is_still_verbatim: the parser reads the pattern to decide what to emit and never rewrites the block, so the content-hashed single file of requirement:component-asset-extraction is untouched and this costs nothing at the delivery layer
    two_components_sharing_one_file: still fine, because what is emitted into markup is decided per component while the file is only code
  parameters_nest_rather_than_flatten:
    problem: a flat bag mixes capabilities with parameters, so a component taking a parameter named actions or el collides with a framework key
    rejected_reserved_words: reserving the capability names is checkable today and forward-hostile, because the reserved set can only grow and every capability added later breaks an application that used that word as a parameter name
    why_that_matters_here: the bag was chosen partly because a fourth capability is one more key rather than a third argument, and flat parameters would take exactly that property away
    chosen: a nested pattern, so the parameter names sit under one key and the parser reads the inner pattern
    cost: a few characters and a nested destructuring pattern to read, which any real parser gives for free once it is parsing patterns at all
  props_stay_a_snapshot: unchanged by this; the values are the ones at mount, per props_are_a_snapshot_and_not_a_binding below
  props_are_a_snapshot_and_not_a_binding:
    fact: a replace releases and remounts, so setup re-runs with new values, but a swap below the marked root leaves setup mounted holding the old ones
    consequence: a value that can change is read from the DOM at event time, where the markup is the source of truth; a value fixed for the instance is taken at mount
    consistent_with: this object being a handler namespace rather than a reactive surface, which is already recorded above as what an author arriving from Vue will expect and not get
responsibility_split:
  system_tinybind:
    - reserving the attribute and lowering it away, exactly as it does for server-action
    - reading the block well enough to enumerate top-level declarations and a returned object literal's keys
    - refusing an unknown name, a name declared in both namespaces, and a handler on an element inside no component declaring a block
    - emitting the event and handler pair as static markup, so the runtime resolves nothing from a string it did not compile
  this_framework:
    - the generic bind at mount and the rebind on a swap, which requirement:component-script-event-binding specifies
    - the diagnostics channel a name unresolved at runtime reports through, which stays necessary because a partial check is not a total one
  precedent: the same division api:page-action-endpoint already runs on, where upstream resolves the symbol and this framework supplies the address and the client half
constraints:
  - the name in the markup is a static literal, so no value a request or a payload carries decides which function runs
  - a handler is a function reached by name, never an expression evaluated at bind time
  - a project declaring no handler is unaffected in its bytes and in its runtime behaviour
naming_the_signal_registration:
  chosen: onSignal, decided 2026-08-11
  candidates:
    on:
      buys: shortest, and it is what scope.on is called today
      costs: a bare on destructured into a function's scope is a generic identifier, and it now collides in meaning with the on-click attribute this catalog just chose for DOM events, so an author would reasonably read on("click") as a DOM binding when it registers a framework signal
    scope:
      buys: keeps today's object whole, so the concept and the documentation do not move
      costs: the bag is the scope, so this nests the scope inside itself, and it spends a level of indirection on history alone
    signal:
      buys: it names the thing, and signal is the vocabulary system:tinybind and requirement:client-signal-registry both use
      costs: it collides with AbortSignal, which is an ordinary thing for a component script to have in scope
    onSignal:
      buys: the vocabulary without the AbortSignal collision, and it reads correctly at the call site
      costs: longer, and it is a third spelling beside registerEvent on the published surface and scope.on in the guide
    subscribe:
      costs: api:client-update-api already has a subscribe for update outcomes, and requirement:client-signal-registry says outright that the two surfaces are not merged, so reusing the word invites exactly that reading
  why: the only cheap alternative is the one our own attribute naming just made ambiguous, and its failure is silent — on("click") registers a signal by that name and never fires, with no error
the_top_level_fallback_is_not_worth_keeping:
  decided: dropped 2026-08-11
  candidates:
    keep_it:
      buys: a stateless handler needs no per-instance closure, and a top-level function stays reachable from a unit test that never calls setup
      costs: two lookup sites, a collision rule to state and check, and a standing invitation to the module-scope trap the component scripts guide exists to warn about
    drop_it:
      buys: one rule with one place, and no collision to adjudicate
      costs: one word
  the_argument_that_settles_it: a top-level declaration is still available, because an author declares the function there and returns it by name; the fallback saves the return and nothing else, while charging a lookup rule and a shared-state hazard for it
  reversibility: adding it later is a widening and breaks nothing, where removing it later breaks every project that used it, so this is the direction that stays open
  side_effect: every handler is then reachable from setup's own scope, so anything the bag carries is in reach of every handler without threading
absence_in_the_props_channel:
  what_it_is_not: a Go zero value, which JSON carries faithfully; 0 goes out as 0 and arrives as 0, and whether that means unset is the application's own question in Go too
  corrected: 2026-08-11; an earlier revision here framed it that way and it does not survive reading the grammar
  what_it_is: the template language has an optional type, so a parameter can be absent as a state distinct from holding a zero, and a props channel has to say which JSON that becomes
  the_house_answer_already_exists: the attribute context of system:tinybind omits the whole attribute when an optional is absent, so absence removes rather than emits
  applied_here: an absent optional omits its key, and the destructured binding is undefined, which is what a JavaScript reader expects from a missing property
  rejected_null: it diverges from the rule the language already set, and it leaves JavaScript with two absences to test for where one would do
  what_stays_the_author_s: distinguishing a present zero from an absent optional is expressible because the type is, and doing it with a plain int is not, which is a Go modelling question rather than a channel one
```
