---
id: requirement:tinybind-update-composition-seams
type: requirement
title: TinyBind Update Composition Seams
---
Seams system:tinybind owns that this framework needs opened before the update capabilities compose cleanly; v0.3.3 answered the five raised against v0.3.1 and v0.3.5 answered the client ownership round, leaving the attribute prefix as the one item still open.

```yaml
owner: system:tinybind
raised_against: v0.3.1
answered_partly: v0.3.2, which settled the documentation defect and carried head on the action response
answered_fully: v0.3.3
status: five closed by v0.3.3; the client ownership question answered by v0.3.5 and settled on this side; the attribute prefix is the one item still open
kept_for: the record of the round and of two corrections this framework had to make to its own findings
as_built:
  live_token: its own negotiated mode, so subscriptions stay open only in live mode; the validator question answered as deliveries carrying none and the opening delta carrying them
  adopted_from_here: the done-versus-retry distinction, the build identity on the opening record, and resetting the attempt count on a healthy close, all offered as input and taken
  found_while_settling_it: the module's live entry had set subscriptions unconditionally, so an ordinary navigation delta on a live route never terminated
  redraw_head: covered from both sides without changing the body, by a registry-level required set for the shell and a response header for the per-redraw case
  asset_set: the required set is on the bound value and readable before rendering; the embedded byte table and caller URL function remain unbuilt
  builtin_elements: registration and lowering shipped, though requirement:module-native-csrf removed the use this framework had for them
  slot_head: the plan now reaches fragments carried in parameter structs, so no report side was needed; the existing rejection logic sees them
  cost_elsewhere: the hyphenated element namespace closed, which requirement:custom-element-registration carries
follows: requirement:tinybind-runtime-ownership, answered in full by v0.3.1
not_blocked: nothing here stops adoption; one item is cheap now and expensive after either side ships
live_mode_convergence:
  priority: first, and the only item that is a wire contract rather than a missing feature
  what_we_did: decision:update-runtime-convergence put this framework's live delivery on the shared render header, as 'live;v=N', tested before delegating to the module
  what_we_relied_on: the documented rule that an unrecognized mode resolves to a complete document, which makes an unknown token inert rather than an error
  first_reading_was_wrong:
    assumed: the module had reserved 'live;v=N' and this framework had taken it
    checked: no live token is parsed anywhere in the module's Go; the live entry point delegates to the streaming navigation entry, which serves live deliveries under the navigation mode as delta records
    so: the published token names a planned mode the code never reads, and the two open questions on the module's own reconnect requirement are the mode spelling and whether the body is the navigation delta or a delivery stream
    revised_position: this framework has shipped the feature the module has designed, so the useful contribution is an implementation to converge against rather than a request for a name
  same_intent: re-request the same page, re-execute the chain, and resume delivering to the live regions the client already holds; whole-region state rather than increments, so no cursor, no event log, no replay; positional boundary ids reproduced by a re-render; navigation supersedes; a version mismatch falls back to a document
  same_two_phase_shape:
    correction: an earlier reading of this entry treated the format change between the initial delivery and the reconnect as a divergence; it is not, and both sides arrived at it for one reason
    initial: chunked, inside the document response, framed for the HTML parser exactly as an async completion is
    reconnect: header-selected and read by script, so the parser framing has nothing to trigger and a record is the ordinary shape
    stated_identically_on_both_sides: api:html-boundary-protocol already says past the initial document there is no parser reading, which is the module's own reason too
  transport_is_not_a_difference_either:
    second_correction: this entry then claimed the module holds the document response open while this framework finishes it; that is also wrong, and v0.3.2 checked it against the code
    what_the_module_actually_does: a live boundary settles in place through the blocking op, so the document response commits first content and finishes, and a second connection carries the deliveries
    which_is_this_framework_exactly: the document render takes one delivery per live boundary and ends with a terminal marker, then the client opens a separate connection
    where_the_misreading_came_from: the module's first-delivery-inline behaviour, where a live boundary's first value rides the document render as an await completion, was read as the response staying open afterwards
    already_argued_upstream: the module's own live transport decision rejects the held-open document response, on the four grounds this framework re-derived independently, and now records that a downstream reached them on its own
  what_actually_remains:
    the_token:
      finding: no live token is parsed in any code on either side, and the module's shipped runtime sends the navigation token for both the first connection and every reconnect
      so: the module's live mode has never had a spelling, and this framework's live token names a mode nothing upstream reads
      module_position: filed as a must-priority requirement, whose recommendation is a live token rather than the navigation one, because the two differ in duration and termination and cannot be routed, timed out, or bounded separately while they share a name
      agreement: that recommendation is this framework's existing choice, so convergence is likely rather than contested
      remaining_substance: whether a live body carries validators; the module writes one per record and this framework deliberately carries none
    control_vocabulary: this framework distinguishes a stream closed because every source finished from one closed healthy at a lifetime bound, with a retry hint on the latter, plus an open record carrying the build; offered as input to the token work rather than as a defect
    backoff: linear and attempt-bounded upstream, exponential with jitter here and reset on a healthy close, so a working screen closed at its bound does not stall
  documentation_defect:
    what: the guide's mode table published the live token and its availability table marked live delivery and reconnection available, while no live token was parsed and the requirement still listed the spelling as open
    why_it_matters: this framework built against the published table, which is how it arrived at the same token by a route the module did not intend
    resolved: v0.3.2, where both tables now describe what the code does and say plainly that the live token is designed and unimplemented on both sides
  ask:
    settle_the_body: decide whether live reconnect reuses the navigation delta body or a delivery stream, since that decision is what makes one token mean one contract
    take_this_as_input: the control vocabulary, the done-versus-retry distinction, and the reset-on-healthy-close backoff are shipped here and are offered rather than asserted
    or_reserve_a_space: failing convergence, a documented caller-owned token space, or extra modes declared on the options and reported back as a distinct negotiated mode
  why_now: a token is a wire contract; settling it after either side ships costs a coordinated deploy, which is what the build identity exists to avoid
client_ownership:
  raised: 2026-08-04, while wiring requirement:reloadable-component-endpoint end to end
  priority: highest of this round, because it decides whether the items below are asks at all
  what_forced_it:
    the_endpoint_is_this_framework_s: pw mounts the redraw route, owns the reserved prefix, owns the registry, and routes every refusal to its own error path
    the_client_is_the_module_s: the browser half builds the redraw URL as prefix plus kind plus instance and sends no render mode at all, so the address of this framework's own endpoint is decided in a dependency
    effect: moving that endpoint under the page URL, which is what makes it inherit the page's authentication rather than needing a protection pattern of its own, cannot be done here at all
  what_the_audit_found:
    one_file: the module ships exactly one browser asset; htmlbind ships none, so this is the whole surface
    apply_duplicated: the module's half swaps through its own function and references neither of this framework's, against what requirement:unified-update-runtime records
    live_duplicated: the module implements a live reader, a stream consumer, and a reconnect policy speaking the token this framework defined, beside the ones already built here
    reach_in: the module reads this framework's live handoff header to decide whether a route expects a delivery stream
  positions:
    minimal: make redraw addressing a caller decision, as the action path already is, where the caller issues the request and the module only applies what came back
    proposed_here: the module supplies no browser code at all and stays with HTML fragment production, boundary identity, and the validators its compiler already emits
    why_the_larger_one: every protocol name is already this framework's configuration, so the browser half is this framework's protocol wearing the module's implementation, and each round spends a coordinated release on something one side could decide alone
    cost_named_honestly: the module's asset is roughly two and a half times what this framework's own half is, and the weight is in form-state reconciliation against per-control defaults, composition deferral, preserved islands, history, scroll, focus, supersession, and the fall-back-to-navigation rule on every failure path
    line_to_draw: whether the module keeps the server halves and publishes the wire format as a specification, or the transport moves whole and htmlupdate is retired
  reverses: the keep_pw_runtime_only rejection of decision:update-runtime-convergence, whose stated reason was the cost above; what has changed is that the coupling cost was not counted then and has since been paid once
  decided: 2026-08-04, that every byte running in a browser becomes this framework's
  not_a_reversal_upstream:
    found: the module already decided the caller owns the browser script, and records its own shipped runtime as a milestone deviation whose exit condition is the default shipping none
    so: the ask is to finish a transition already defined rather than to change a position, which is how the request is written
    blocking_item: the wire contract is listed as the module's to publish and its form is still an open question there, so a caller-written client today infers the protocol by reading the module's JavaScript
  request_written: docs/tinybind-go-client-ownership-request.md, against v0.3.3
  answered: v0.3.5, on every ask
  what_shipped:
    wire_contract: published as docs/httpbind_update_wire_contract.md, which is what a caller-written client needs and what the module had listed as its own to write
    redraw_addressing: a negotiated redraw request mode with the component on kind and instance headers, and an Options entry answering from a request rather than from a mounted route, so the caller picks the URL
    mount_narrowed: Mount takes no registry and registers the asset alone, because a redraw is no longer an endpoint the module owns
    asset_default: ServeRuntime is off by default and exactly one of it and CallerOwnsRuntime must be set, which retires the milestone deviation the module had recorded against itself
    unasked_and_welcome: validators are now seeded with the build identity, so two builds cannot produce comparable digests where the build header was dropped in transit
  what_this_framework_did: moved the redraw to the page URL, retired the reserved-prefix route, and added an explicit handler entry naming the components a URL answers for
  still_ships_javascript: the module's browser runtime is unchanged, and this framework no longer merges it
  closed_here: 2026-08-04, when the client half was written against the published contract; the module ships a reference implementation and this framework ships its own, which is the shape the module's own client runtime ownership decision always described
  nothing_further_asked: the remaining seam items are unaffected, since none of them is about the browser
head_on_redraw_and_action:
  what_works: the navigation delta carries the merged head, so a component appearing for the first time installs its tags before its markup lands and never flashes unstyled
  action_half_shipped:
    when: v0.3.2
    what: the action response now collects each written region's own head contributions, deduplicated across the set by the same merge rule
    why_it_was_only_half_missing: the browser already installed a delta's head before applying operations, so only the server was never filling the field
    closes_for: requirement:action-response-update
  redraw_half_open:
    what: the redraw handler still writes a rendered subtree and response headers with no head
    consequence: redrawing in a component whose stylesheet or script is not already on the page renders it unstyled, which is the exact failure the navigation path added the field to prevent
    affects: requirement:reloadable-component-endpoint
    our_workaround: ensure every asset a redrawable component needs is already on the page, which nothing checks and nothing reports
    module_position: recorded as a requirement rather than settled
  ask: carry the same head field on the redraw response, or state that the caller guarantees presence and give it the means to know, which is the next item
static_required_asset_set:
  already_designed: the module's own component asset requirement carries the statically known required set, the embedded table, and the caller-supplied URL function
  why_it_matters_here: it is what makes the item above solvable without a mid-swap fetch, since the document can carry every asset a later delivery might need
  status: unimplemented, and its own hardest property is the one this framework needs
  ask: priority rather than design; the redraw and action gap is what makes it concrete
builtin_element_registration:
  blocked: the framework-supplied csrf element this catalog planned, until requirement:module-native-csrf removed the need for one
  status: designed upstream across a registration requirement, a lowering requirement, and a value provider, and none is implemented as of v0.3.1
  agreed_example: csrf-token is the worked example in the module's own design and was the first example in this framework's report, reached independently
  not_blocked: a synchronous external returning html, with the render context on its Go implementation, is a working interim shape, so this costs one declaration per template file rather than the feature
  ask: unchanged from the earlier round; the registration seam is the piece with no substitute
csrf_token_rotation:
  withdrawn: raised as an ask, then answered on this side before it was sent
  the_gap_as_raised: a token reaching the browser once, as head metadata written at render, cannot be refreshed, so a page held across a rotation holds a stale token
  why_it_was_the_wrong_ask: the defect was in this framework's chosen transport rather than in the module, which had picked a page-embedded token with no refresh channel
  resolution: read the token from a cookie at request time and refresh it by set-cookie, which is the shape Django, Laravel, and Spring's SPA configuration all use; it needs no module change and works identically on the navigation, redraw, and action paths
  independent_of: the head gap above, which is what makes it worth stating separately rather than folding in
  what_stays_true: a form rendered before a rotation is refused after it, which is correct and is what every comparable framework does
  module_open_question: a delta-response token refresh header remains listed upstream; this framework no longer needs it
routetree_drops_the_attribute_prefix:
  found: 2026-08-03, while wiring requirement:navigation-delta-rendering
  what: the generator option naming the boundary attributes reaches the templates path but not routetree, whose compileTemplate builds its generate options without it and exposes no option of its own
  effect: a page tree's components keep the module default while a registered-router template takes the configured prefix, so one document can hold data-pw-id boundaries beside tb-boundary placeholders
  why_it_matters: this is the split naming the prefix option exists to prevent, and v0.3.1 closed it for the render path only
  workaround_here: use the module default on both paths, because one agreed spelling is worth more than the framework's brand; api:html-update-options records the choice
  ask: thread the prefix into routetree's template compilation, or give routetree the same option
  size: small, and it is the last place a document can end up with two spellings
fragment_head_from_a_parameter:
  the_defect: binding copies only the plan's own contributions, so a fragment passed into a component through its parameter struct contributes no head
  why_it_matters_here: decision:fragment-head-rejection refuses a fragment response carrying head contributions, and this defect makes that check incomplete for exactly the cross-file composition case, so a slot-supplied component's styles are dropped rather than reported
  accepted_upstream: v0.3.2 filed it as its own requirement, plus a second one for what a fragment response owes a caller it cannot deliver to, rather than settling either in that release
  ask: unchanged; walk parameter-carried fragments when merging, or report them so a caller can refuse rather than lose them
carried_forward:
  live_mode_plan_slice: requirement:live-mode-plan-slice, since a live render still executes the whole composed chain and pays it per reconnect
  liveness_signal: requirement:live-boundary-liveness-signal, since nothing states which boundary is live and this framework keeps its own bookkeeping
  status: raised in earlier rounds, unchanged by v0.3.1, and listed so a round does not read as if they were resolved
not_asked_for:
  - the delta protocol, the validators, the manifest encoding, or the operation kinds
  - anything settled by v0.3.1, which answered its round in full
  - a second convergence of the live transport itself, which is this framework's decision rather than a module gap
acceptance:
  - the live token means one body, whichever side implements it, and the guide describes what the code does
  - a component appearing for the first time in a redraw or an action response installs its assets before its markup lands
  - a caller can read a chain's required assets before rendering starts
  - a template writes one registered element for a CSRF token
  - a slot-supplied component's head contributions are delivered or reported, never dropped
  - one attribute prefix reaches every generation path, so no document holds two spellings
```
