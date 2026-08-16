---
id: decision:message-code-shape
type: decision
title: Generated Message Shape
---
Generated code scales with the number of messages, generated data scales with messages times locales, and adding a locale emits no code at all.

```yaml
status: proposed
source: user question 2026-08-16, asking whether this becomes a per-locale instruction set selected by switch
shape:
  typed_wrapper: one Go function per message; scales with messages only
  segment_table: one row per message, locale, and variant; data, not code
  renderer: one loop shared by every message
  locale_selection: an array index, not a switch arm per locale
  template_side: one call per reference
segment:
  members: a literal run or an argument slot
  encoding: a small struct with no interface, so TinyGo emits no indirect call
  arguments: converted to strings by the typed wrapper with concrete code, never boxed
  number_grouping: an int argument is grouped for the locale from a table of separators; digit shaping, currency, and ordinals are not modelled, because a message argument is a count rather than a formatted money value and half of currency would be worse than none
  untabulated_locale: the comma-and-period convention rather than an error, since a number grouped the wrong way is legible and a page that failed is not; plural selection reads the value and never the formatted string
why_not_straight_line_code_per_locale:
  shape: a switch on locale whose arms are write sequences
  cost: code grows with messages times locales, and code does not compress or share the way a data section does
  scale: a five-locale application with five hundred messages emits two and a half thousand sequences
  what_it_would_buy: one fewer indirection per segment, against a target where WASM size is the constraint
why_not_a_format_string:
  shape: a per-locale format string passed to fmt
  why_not: fmt is reflective, which api:typed-external-function forbids and which TinyGo pays for in binary size
  smallest_data: yes, which is why it is named rather than ignored
placeholder_reordering: free, because each row lists its own segments in its own order, so no positional index scheme is needed
escaping_stays_where_it_is:
  fact: a message returns an unescaped string and is escaped once, with its arguments, by the template's existing context rules
  effect: the same message is correct in text, attribute, and URL contexts with no message-side knowledge of context
  no_double_escape: literals are stored unescaped, so escaping the concatenation is the only escape applied
  no_exception: policy:message-rich-text returns segments rather than writing markup, so every form is escaped upstream and the boundary claim of decision:upstream-message-surface holds with no carve-out
  what_this_replaced: a design in which rich text wrote to the stream and discharged the escaping obligation downstream
two_generated_shapes:
  string_form: func(loc pw.Locale, args...) string, for a message with no hole; allocates nothing when it takes no argument
  segment_form: a function returning an ordered list of text runs and hole markers, for a rich-text message only
  arguments: interpolated inside the function in both shapes, so a run is complete before upstream escapes it
  cost_of_the_second: one slice per rich-text reference per render, which is why it is the rich-text path rather than the general one
id_to_symbol_is_a_table:
  cause: an ID may carry a hyphen, per decision:message-id-assignment, so it is not a Go identifier and no naming convention can derive the symbol
  form: the mapping is supplied to generation as data, carrying package, name, and the declared parameter list
  params_are_load_bearing: a reference names its arguments and Go takes them positionally, so the declared list is what fixes call order
  what_it_buys: the slug policy stays downstream, and renaming a generated symbol is a data change rather than a convention upstream has to match
locale_argument_comes_from_a_binding:
  mechanism: the message context binding of data:locale-bindings supplies the leading parameter
  why_not_read_from_context: a reference that named nothing would be invisible to the cache-key walk, so a cached component carrying a message would not key on its locale
  effect: a reference is an ordinary reader of a binding, and no rule about messages appears anywhere in the cache
argument_type_checking_is_not_generation_time:
  delivered: arity and argument names, checked against the supplied table
  not_delivered: the fit of an argument value to the parameter's Go type
  consequence: a wrong type surfaces as a compile error in the generated package rather than as a diagnostic at the reference
  why_accepted: it matches the syntactic path external declarations already take, and the stronger option makes a template compile depend on a message package that compiles
zero_argument_messages: return the table string directly, so the common case allocates nothing
plurals:
  selection: the typed wrapper computes the category and indexes the row
  rules: generated for declared locales only, so a single-locale project collapses to a constant
  category_set: a property of the target locale, which is why variants live in the catalog and not at the reference
fallback_is_flattened:
  when: generation, because the chain is static
  effect: no table row is empty and no runtime fallback branch exists
  consequence: what happens on a missing translation is decided entirely by the build severity of data:i18n-config
composition: requirement:catalog-composition is flattened by the same pass, so a composed catalog costs what a single one costs
generation_order:
  constraint: catalog, then message package, then template compile
  why: decision:upstream-message-surface item F type-checks a reference against generated signatures
  affects: flow:generation-pipeline, and the watch scope of decision:developer-loop-watch-scope, since a catalog edit must retrigger template generation
```
