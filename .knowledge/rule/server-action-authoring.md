---
id: rule:server-action-authoring
type: rule
title: Server Action Authoring Rules
---
A server action is a public POST endpoint that a template can name, so what an author decides is what the handler will accept from anyone and which element kind can still invoke it when the runtime is not there.

```yaml
source: api:page-action-endpoint, requirement:action-invocation-runtime, requirement:scriptless-action-forms
inherits: the reachable-surface rule of api:page-action-endpoint, which publishes every exported handler-shaped function in a route package whether or not a template mentions it
the_address_grants_nothing:
  fact: the hash hides structure and is not a capability token
  consequence: every handler authenticates and authorizes its own caller, exactly as an ordinary route does
  what_the_framework_does_supply: policy:csrf-protection over the prefix and policy:authenticated-path-protection patterns, both of which are configuration a project can get wrong
  the_test: whether the handler would be safe if its URL were printed in a newspaper, because an opaque path in a page's markup is closer to that than to a secret
  exported_is_published: a helper that happens to take a writer and a request becomes an endpoint by existing, so a function not meant to be called over HTTP is lower-cased rather than left exported
choose_the_element_for_the_client_you_owe:
  form: works with and without the browser runtime, per requirement:scriptless-action-forms
  bare_element: works only with the runtime, because nothing in the markup invokes it and no lowering can change that
  rule: a mutation a project's own acceptance requires without script goes on a form
  applies_to: every project claiming requirement:classic-web-acceptance, which is the criteria list saying pages work without the Popcorn Web browser runtime
  portability_is_one_way: moving a form to a bare button loses the scriptless path silently, and moving a bare button to a form loses nothing
two_shapes_chosen_by_who_calls:
  raw: an ordinary func(http.ResponseWriter, *http.Request), reachable from a form, a gesture and a script alike, admitted by its shape and published by existing
  typed: any signature, declared per decision:typed-action-declaration and reachable only from a script calling it by name per decision:typed-action-is-call-only
  how_an_author_chooses: by asking who calls this rather than what it should return, which is the question that has an answer
  a_form_needs_the_raw_one: a template naming a typed action is a generation error, since a native submit would be shown a value it cannot render
the_handler_owns_its_whole_response:
  applies_to: the raw shape; a typed action's response is decided by generation, which is what narrowing its caller bought
  shape: an ordinary func(http.ResponseWriter, *http.Request), testable with httptest and registered by nothing the author writes
  three_answers: the regions requirement:action-response-update carries, a redirect, or an ordinary response
  branch_point: one predicate, so the update path and the ordinary path cannot drift apart
  writing_nothing: on the scriptless entry point that means the default redirect back to the page; on a fetch it means an empty response, which is a bug rather than a shape
  status_is_real: a rejected submission returns 4xx and the regions it carries are the validation errors, which is what policy:validation-errors decides the content of
read_input_through_the_binder:
  available: api:request-binding works inside an action, so a handler reads its input with pw.Parse like any other
  why_it_matters_here: the request model discovery of system:tinybind recovers the input type from the bind call, which is what lets a form control name be checked against a Go field at generation
  a_handler_that_never_binds: offers no type to check against, so the check is skipped and reported rather than passing silently
  path_parameters: readable only from the entry point that carries them, which decision:action-entry-point-selection is what settles
never_make_the_action_a_dispatcher:
  forbidden: a handler whose behaviour is selected by a name, a table key, a path, or a callable the request supplied
  why: the same argument rule:client-event-authoring makes about a registered client handler, one layer down; one published endpoint that dispatches by payload grants whatever that argument can name
  test: whether adding a value to the request could change which code runs without editing any Go, and if it can, one endpoint published everything reachable from it
one_action_one_mutation:
  rule: an action performs its mutation and answers; it does not become a general write API for a page
  why: it is a page's implementation detail, published in no OpenAPI document and versioned by nothing, so a caller outside the page has no contract to hold
  where_a_real_API_goes: the ordinary typed routes, which is what api:api-response and the OpenAPI document already serve
caching:
  rule: a component holding an unsafe form cannot be output-cached, enforced at generation across the call graph
  reason: a stored body would hand one session's token to the next visitor
  shape_it_pushes: split the cacheable list from the form that carries the token, and compose both in the page
side_effects_belong_to_the_action:
  rule: only the action mutates; the redraw of requirement:reloadable-component-endpoint and the delta of requirement:navigation-delta-rendering are GETs that must stay side-effect free
  why_it_is_stated: an action commonly refreshes several regions, and doing the mutation in whatever re-renders them would repeat it on every reload
  transaction_boundary: explicit, per api:server-action; the handler opens and closes it rather than inheriting one
what_an_author_still_writes_themselves:
  today: the invocation, because requirement:action-invocation-runtime is not built; the docs say so and this rule does not pretend otherwise
  after_it_lands: nothing for a gesture; a component script calling a handler with no gesture still writes its own fetch until the client stub question is answered
```
