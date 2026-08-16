---
id: decision:message-id-assignment
type: decision
title: Message IDs Are Proposed, Not Derived
---
The extraction tool proposes an ID from the source text when it mechanically can, a human confirms it, and drift is judged against the recorded original rather than against the ID.

```yaml
status: proposed
source: user discussion 2026-08-16
id_shape: declared scope of decision:message-scope-declaration plus a name, so about.welcome-user
lexical_form:
  accepted_upstream: dot-separated segments of word characters and hyphens, no whitespace, and no segment starting or ending in a hyphen
  hyphen_was_a_question: the expression lexer reads a hyphen as subtraction, and the leading case is the trap; a segment may not begin with one because "{t x -y}" would silently lex as arithmetic
  what_it_bought: a slug reads as a slug rather than adapting to Go's lexer
  what_it_costs: an ID is not a Go identifier, so decision:message-code-shape supplies the symbol mapping as data rather than deriving it
assignment:
  automatic: a slug derived from the source text, offered as a proposal
  interactive: api:cli-i18n asks, because it is already an interactive rewrite
  manual: a human may name it outright
  frozen: the ID never changes when the text later changes; renaming is an explicit operation
best_effort_is_the_point:
  problem: a slug from Japanese needs kanji readings, which needs a morphological dictionary
  rejected: bundling a dictionary in the host tool, which requirement:cli-distribution would carry for every user of every project
  also_rejected: forcing a project to declare an ASCII development language so a slug is always derivable, which makes a Japanese-first project write English it does not otherwise need
  accepted: kana romanizes mechanically, kanji does not, and the tool falls back to asking
  consequence: naming a message costs what naming a variable costs, which is less than writing the text in a second language
  why_readability_is_not_critical: the catalog carries the source text beside the ID, so a translator reading the catalog is not relying on the ID; the ID is for the developer grepping code
drift_detection:
  rejected_criterion: whether the slug still matches the text, which is a natural-language judgement and produces false warnings until nobody reads them
  chosen_criterion: whether the source-locale text differs from the original recorded at assignment, which is a string comparison
  recorded_as: a snapshot field the tool writes into the catalog entry, never edited by hand
  one_event_two_effects:
    - mark every other locale of that message stale, which is the fuzzy signal a translation needs
    - suggest revisiting the ID
  why_they_share_an_event: both are triggered by exactly "the source text changed", so no second mechanism appears
extraction:
  marks: ordinary HTML attributes, so no grammar is needed
  element_text: "<p i18n>text</p>"
  attribute_value: "<input placeholder=\"text\" i18n=\"placeholder\">"
  go_sources: a pw.T call site, rewritten to the generated typed call
  rewrite: the tool assigns an ID, writes the text into the catalog, and replaces the source, per decision:upstream-message-surface item H
  transient: a mark does not survive the rewrite
collision: a suffix, reported rather than silently applied
```
