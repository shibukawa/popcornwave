---
id: api:cli-i18n
type: api
title: pw i18n
---
One subcommand moves text between sources and catalogs, and every operation it performs is one a human confirms rather than one it infers.

```yaml
placement: api:subcommands, beside api:cli-generate
built: every operation below, 2026-08-16
interchange_format:
  chosen: gettext PO
  why: the mapping is exact and needs nothing invented — msgctxt carries the ID, msgid the source text, msgstr the translation, which is the same asymmetry decision:message-source-of-truth records
  fuzzy: a source text differing from its recorded snapshot is exported fuzzy, so a PO tool shows the translator exactly what decision:message-id-assignment marks stale
  not_xliff: it expresses the same thing with a schema and a parser dependency, for a round trip every translation tool already does in PO
operations:
  extract:
    rewrites_in_place: the marked range becomes a reference, the mark attribute and the whitespace before it are removed, and a file with no scope declaration gains one
    declines_rich_text: an element whose content is not one run of text is reported rather than converted, because a hole name invented by a tool is one no translator can check
    shows_every_decision: the ID proposed for each string, and each mark it declined with the reason
    reads: marked text in templates and pw.T call sites in Go, per decision:message-id-assignment
    proposes: an ID per marked string
    writes: the catalog entry, the assignment snapshot, and the rewritten source
    interactive: it asks when a slug cannot be derived mechanically, which is every kanji-bearing string
    idempotent: an already-extracted string carries no mark and is not seen again
    template_rewrite:
      offsets: the start and end range system:tinybind reports for a text node or attribute value, per decision:upstream-message-surface item H
      range_is_source_not_content: an escaped brace contributes one character of text and two of range, so replacing the range replaces the escape, which is what a rewriter wants
      alternative_not_taken: parse, edit the tree, and print through the upstream printer, which avoids reimplementing quoting and attribute shape at the byte level but normalizes the whole file
      why_offsets: a project that is not already formatted would see unrelated diff noise on extraction, and extraction is the moment a reader is least able to judge it
      revisit_if: the splicer accumulates quoting cases, since the printer is the correctness-first option and both remain available
  check:
    built: yes
    reports: undefined references, unused IDs, placeholder mismatch, missing plural categories, missing translations, and stale translations
    the_question_a_build_cannot_ask: whether a declared message is still reached; generation asks only whether every reference resolves, so an orphaned message is invisible to it
    reads: the upstream reference report over every template of the declared purposes
    unused_is_a_warning: removal is a human judgement, since a message may be reached from Go or kept for a page about to land
  sync:
    same_command_as: check, named for the direction a translator thinks in; one implementation, because two would drift
    reads: the upstream reference report, which answers before any symbol table exists, which is the order this needs since the table is built from it
    partial_input_is_reported: a bare reference in a file declaring no scope comes back with an empty ID rather than failing, so one mistake does not hide the rest of the tree
  rename:
    what: an ID within one scope
    carries: every locale's translation and the snapshot, by editing the key line rather than re-marshalling, so comments and ordering survive
    across_scopes: refused, because it moves the entry to another file and a diff showing only one side hides half the change
    why_a_command: a rename orphans translations, and doing it by hand loses them silently
  export:
    to: XLIFF or PO, for a translation service
    source_element: filled from the declared source locale, which is what those formats expect
    generated: the exported file is not committed, per policy:generated-artifacts
  import:
    from: the same formats
    writes: target translations only, never the source locale and never an ID
    conflict: reported, never merged by similarity
does_not:
  - assign an ID without showing it
  - guess whether an edited string is a new message or a changed one; it asks
  - write a translation for a locale, including a machine translation
  - reformat a catalog it did not change
determinism: no clock, no network, no locale-dependent sorting of output
```
