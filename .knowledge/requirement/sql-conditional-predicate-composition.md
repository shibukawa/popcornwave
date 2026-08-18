---
id: requirement:sql-conditional-predicate-composition
type: requirement
title: Conditional Predicate Composition
---
A .pw.sql predicate assembled from conditions that may each be absent emits valid SQL for every branch combination, so no author manages an operator, a comma, a parenthesis, or a clause keyword whose correctness depends on which sibling survived.

```yaml
owner: system:tinybind, which owns the grammar, the compiler, and the statement builder
consumed_by: flow:sql-generation
status: delivered in system:tinybind v0.5.15, consumed here 2026-08-18 by the go.mod bump
mechanism_owner:
  where: the system:tinybind catalog, under the boundary-joiner-inference decision and the predicate-group-elision rule, whose ids belong to that catalog rather than this one
  why_not_restated_here: those entries carry the frame protocol, the exactness rules, and the acceptance list in full; a second copy on this side would drift against the release that owns it
  what_this_entry_keeps: why Popcorn Web asked, what the release changed about the ask, and what the documentation teaches
problem_it_closed:
  before: the operator joining two conditions was text the author wrote, correct only for the branch combination they had in mind
  failures: a leading operator left "WHERE AND x", an operator inside parentheses left "( AND y)", and an all-conditional predicate left a bare WHERE before ORDER BY
  why_the_author_could_not_fix_it: whether a joiner is needed depends on which sibling survives, which only the builder knows
  scale: branch combinations are exponential in the conditions, so a hand-written operator is correct by luck past two
upstream_request: docs/tinybind-go-conditional-sql-joiner-request.md, sent against v0.5.14
what_the_release_changed_about_the_request:
  joiner_recognition_is_broader: the request asked for an operator adjacent to a node whose alwaysEmits is false; the release makes every AND or OR at the innermost open group's item depth a joiner, because the narrow rule misses the operator joining a parenthesis group to its sibling, where the closing paren stands between the operator and the elidable node
  comma_clauses_landed_together: the request deferred them; SET, VALUES, an INSERT column list, ORDER BY, GROUP BY, FROM, WITH, WINDOW, USING, and PARTITION BY are managed in the same release, with SELECT and RETURNING left as text because a conditional result column is already refused
  join_on_was_already_classified: the request raised ON as an open question on the premise that it was not classified boolean; it already was, so ON needed no new classification
  case_is_excluded_rather_than_modelled: inside a CASE region nothing is withheld at all, so such a region renders exactly as before; a fragment there that can emit nothing is a diagnostic naming CASE, which is the request's stated preference
  a_conditional_clause_keyword_stays_legal: "{if a}WHERE x = {x}{/if}" is a pre-existing idiom and still renders unchanged, so the unbalanced-branch error narrows to parenthesis nesting
  byte_identity_became_structural: a body with nothing elidable is planned as before, so not one group call reaches its generated code; the compatibility claim no longer rests on the frame protocol being output-neutral
also_in_the_same_release:
  insert_item_agreement: an INSERT whose column count and value count can disagree on some branch is a generation error, which is new surface a .pw.sql author can hit
  reserved_parameter_names: ctx and db are refused as parameter names because they are the context and executor of every generated function; err and result stay available
verified_here:
  method: generated and ran the documented templates against v0.5.15 rather than reading the release notes
  wheres: the four-condition predicate renders the full clause with authored newlines, drops the leading operator and the emptied parenthesis group with its AND when one condition survives, renumbers the surviving placeholder to $1, and omits WHERE entirely when none survives
  inserts: a paired conditional column and value render together and vanish together
  diagnostics: the BETWEEN split, the CASE fragment, the INSERT count disagreement, the all-conditional SET, and the ctx parameter each fail generation with the construct named
documentation_stance:
  canonical_form: the operator between the two conditions, in the enclosing text, because that is where it sits in the finished statement and it is what lets the source read as its own output
  in_branch_form: works identically and is not deprecated; the pages note it reads as part of one condition when it joins two
  not_tool_enforced: no formatter normalizes between the two, because moving a token across a branch boundary would change the AST
  pages: reference/sql-templates and guides/storage/queries in both locales, the reference carrying the managed-clause list and the guide two cases and the boundary
popcorn_web_scope:
  - consume the release and regenerate, per decision:tinybind-sql-runtime
  - teach the canonical form
  - add no SQL parser, no predicate analysis, and no builder
```
