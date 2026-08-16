---
id: decision:upstream-message-surface
type: decision
title: Upstream Message Surface
---
system:tinybind gained message syntax, symbol resolution, implicit bindings, and a parse surface in v0.5.13, and learned no i18n semantics doing it.

```yaml
status: accepted upstream, consumed downstream as proposed
requested: 2026-08-16
delivered: system:tinybind v0.5.13, recorded there under its own template-message-surface concept and the six requirements below it
upstream_ids_are_not_referenced_here: an upstream catalog is a separate namespace, so this file names system:tinybind and describes what it holds rather than linking into it
boundary:
  upstream: how a message reference is spelled and what it resolves to
  downstream: catalogs, locale resolution, plural rules, fallback, ID assignment, extraction policy, and code generation
  test: a reader of the upstream source who can tell these features exist for translation is reading a boundary drawn in the wrong place
  held: escaping was not changed in any form, which is the claim the rich-text design was reworked to preserve
dialect_scope:
  intended: the html output kind of concept:template-source-dialects
  as_built: the recognizer sits in the shared body grammar, so every dialect parses a reference and only html resolves one; the sql dialect refuses at analysis, naming the reference
  lesson_worth_keeping: a shared-grammar addition reaches every dialect the moment it parses, so a scope line is a claim each dialect carries rather than a property of where the work was done
items:
  A_scope_declaration:
    delivered: the header form, one per file, resolved in the compiler rather than the parser so a declaration may follow the component using it
    owner_of_meaning: decision:message-scope-declaration
    annotation_fallback: not taken, and recorded upstream as a real substitute rather than a degraded one
  B_reference_syntax:
    delivered: "{t <id>} as a contextual directive, evaluating to a plain string"
    keyword: t, so no fallback spelling was needed
    ids_may_carry_hyphens: dot-separated segments of word characters and hyphens, no segment starting or ending in one
    arguments_are_named_at_the_reference: "{t item-count, n: count}, so a placeholder is bound explicitly rather than resolved from surrounding scope"
    represented_as_an_expression: legal in text, attribute, condition, component argument, and val positions with no rule of its own
  C_qualified_ids: delivered, a dot is the marker and a bare name is never also a qualified one
  D_rich_text:
    delivered: a block form whose holes are named by their bound element's tag
    lowering_inverted: the framework returns segments and upstream interleaves them, per policy:message-rich-text
    open_contract_answered_by_removal: the closure signature question disappeared with the closures
  E_implicit_bindings:
    delivered: in full, including the path-segment kind and its collapse
    provider_may_be_typed: so the binding carrying pw.Locale crosses the boundary without upstream learning the type; such a binding cannot be written into markup
    vary_axis: one optional axis per binding, folded into the response vary
    cache: a cached component keys on the bindings it reaches, per data:locale-bindings
  F_symbol_resolution:
    delivered: arity and argument-name checking against a supplied symbol table
    not_delivered: type checking of argument values against the symbol's Go types, deliberately, matching the syntactic path external declarations already take
    consequence: a wrong argument type is a compile error in the generated Go rather than a generation diagnostic
    locale_argument: supplied by a named implicit binding rather than read from context, which is what makes a message reference visible to the cache-key walk
  G_H_parse_surface:
    delivered: MessageRefs reporting scope, written and resolved id, arguments, and position; start and end offsets on text nodes and attribute values
    channel: a package-level report, because a caller needs it before it has anything to pass to generation
    unresolvable_is_reported: a bare reference in a file declaring no scope returns an empty id rather than failing, so reconciling a tree sees all of it
corrections_to_the_request:
  recognizer_rule_was_insufficient:
    claimed: an identifier followed by an identifier is unambiguous
    actual: the existing dispatch is a prefix test, and "{t == x}", "{t > n}", and "{t ?? d}" all carry the same prefix while meaning the parameter
    resolved_by: recognizing the directive only when the entire brace body parses as a reference, with a test covering every shape a parameter named t can take
  the_compatibility_argument_for_scoping_by_kind_was_void:
    claimed: "href=\"/search/{q}\" with an empty q yields /search/ today, so a rule stated over emptiness would change it"
    actual: that template does not compile for any q, because the url type gate runs per interpolated part and refuses a string
    conclusion_survived_anyway: scoping to the binding kind is still right, so a future url-typed interpolation does not acquire collapsing it never asked for
  E_was_not_additive:
    finding: admitting a path-segment binding into a URL attribute amends the rule that keeps a raw string out of one
    justification_accepted: the value is embedder-supplied rather than request-supplied
    condition_attached: the rendered segment is percent-encoded, because a locale resolved from Accept-Language or a path prefix is attacker-influenced by definition
    recorded_in: decision:explicit-locale-in-links, which is where the downstream obligation lands
  offsets_were_not_already_exposed:
    claimed: positions exist internally and only need publishing
    actual: a node carried line and column only, and a rewrite needs a range rather than a position, so both a start and an end were additions
  a_new_expression_kind_is_not_additive:
    finding: every switch over an expression is a place that silently does the wrong thing rather than failing to compile
    instances: binding-scope analysis missed a reference's arguments, three copies of position lookup reported line 1, and the emitter would have dropped a render context an argument needed
    worth_carrying: the same shape applies to any downstream analysis walking these templates
what_downstream_now_owes:
  - the id to symbol table, as data, since an id carrying a hyphen is not a Go identifier
  - the segment list for a rich-text message, per policy:message-rich-text
  - the binding declarations and their values, per data:locale-bindings
  - reporting both directions of a hole mismatch, since both need the catalog
  - percent-encoding is upstream's, but choosing what may become a locale tag is not
pin: system:tinybind moves to v0.5.13; a source using messages or a reference fails to parse on any earlier release
```
