---
id: decision:component-parameter-disclosure
type: decision
title: Component Parameter Disclosure
---
Deriving the emitted parameter set from what `setup` destructured makes a JavaScript destructuring pattern decide what reaches the browser, so this framework states that where an author will read it and treats what comes back as untrusted.

```yaml
source: the disclosure note system:tinybind v0.5.8 leaves to the caller, and decision:component-handler-namespace which chose the derivation
review_gate: proposed
the_hazard:
  mechanism: a name destructured in setup becomes a value in the DOM, readable and editable by anyone holding the page
  why_it_reads_fine_until_it_does_not: pulling out {label} is obviously display, and pulling out {price} beside it looks identical in the source while meaning something else
  what_makes_it_sharp: the author is writing JavaScript and thinking about what their handler needs, not about a trust boundary, and nothing in the syntax says one is being crossed
  it_is_the_same_class_as: the projection hazard rule:client-event-authoring states for a signal payload, where a struct the author named carries a server-only field to the browser because nobody re-read the type
why_the_derivation_is_still_right:
  the_alternative_was_worse: emitting every parameter puts values in the DOM nobody asked to expose, and this at least bounds the set to names an author wrote down
  a_separate_declaration_drifts: two lists to keep in step is the failure this framework avoids everywhere else, and the destructuring cannot fall out of date with the code that uses it
  so: the answer is disclosure being visible rather than the derivation changing
what_this_framework_does:
  says_it_where_it_is_read: the component scripts guide states that destructuring a parameter publishes it, beside the syntax rather than in a security appendix
  names_the_test: whether the value would be acceptable in view-source, since that is exactly where it lands
  untrusted_on_return: a value that came back from the browser is caller input under the same rule api:page-action-endpoint applies to an action's whole request, so it is validated rather than believed
  signs_what_must_not_change: the same answer system:tinybind records for round-trip state on its lowering profile, which is that DOM-carried state is client-editable and must be signed or treated as untrusted
what_it_does_not_do:
  no_allowlist: a second declaration naming what may cross would reintroduce the drift the derivation removed
  no_type_marker: an exposed marker on a Go field would place the decision far from the destructuring that triggers it, so a reader would still have to hold both
  no_redaction: nothing is filtered on the way out, because a framework guessing which fields are sensitive is a guess an author cannot audit
the_type_rule_is_not_the_reloadable_one:
  corrected: 2026-08-12, against v0.5.8
  what_this_catalog_said: reuse the serializability rule of requirement:reloadable-component-endpoint, which refuses a record and a slice
  why_that_was_wrong: that rule exists because a query string must carry the value deterministically, and an attribute holding JSON is not a query string
  what_actually_applies: the rule the script-context JSON insertion already uses, which accepts a record and an array of accepted types recursively and refuses html and a value that has not settled
  why_it_matters_here: a record is exactly what an author destructures a field from, so refusing one would have made the feature much smaller than it is
constraints:
  - a component declaring no script block emits nothing, so nothing is disclosed by adopting the runtime alone
  - the emitted set is derivable by reading the block beside the template, so a reviewer can answer what a page publishes from its own source
```
