---
id: decision:message-source-of-truth
type: decision
title: Message Source Text Lives in the Catalog
---
A template carries the message ID only, the catalog carries the text for every locale, and the editor puts the text back in front of the author.

```yaml
status: proposed
source: user discussion 2026-08-16, on whether an inline original alongside the ID is redundant
question: where the source-language text lives when a message is referenced from a template
chosen:
  template: "{t about.title}, an ID and nothing else"
  catalog: every locale including the source language
  editor: requirement:editor-navigation hover and an inlay hint render the source text at the reference
rejected_alternatives:
  id_plus_inline_text:
    shape: "{t about.title: \"Welcome\"} with the text repeated in the template"
    why_not: the line states the same content twice, and the copy has to be verified against the catalog or it rots
    what_it_would_need: a build check plus a sync command for both edit directions
  inline_text_only:
    shape: "{t \"Welcome\"} with the ID derived from the text"
    why_not: editing the text changes the derived ID, so the binding to existing translations survives only through a similarity heuristic
    where_that_is_acceptable: gettext lives with it; this catalog prefers a stable ID and an explicit rename
  template_owns_source_catalog_owns_translations:
    shape: the source language is normative in the template and the catalog holds only targets, matching XLIFF source and target
    why_not: rich text and plural variants both need the text in the catalog anyway, so the split would hold for plain messages only
    what_survived_from_it: the export of requirement:catalog-composition still fills XLIFF source from the declared source locale
the_cost_this_accepts:
  what: reading a template no longer shows the text, so a UI change is reviewed across two files
  why_acceptable: requirement:pw-language-server already plans hover at stage 3, so the editor restores what the file gives up
  dependency: this decision assumes the language server ships; without it an ID-only template is a real ergonomic regression
  precedent_against: Angular, Lingui, and react-intl keep text inline and pay the redundancy instead
consequences:
  - the source language is one locale among several, so changing which locale is source is a catalog operation rather than a rewrite
  - a diff that changes wording is confined to the catalog, which is where a translator looks
  - policy:message-rich-text is the one form whose structure stays in the template, because markup and text interleave
placeholders:
  spelling: "{name} inside the catalog string"
  binding: named explicitly at the reference, as "{t greeting, name: user.Name}"
  not_resolved_from_scope: an earlier reading had a placeholder pick up a same-named value from the surrounding template; the delivered form names it, so a message is reusable wherever its argument comes from and a reference reads as a call
  checked: arity and argument names at generation, argument types by the Go compiler, per decision:message-code-shape
```
