---
id: requirement:template-formatting
type: requirement
title: Canonical Template Source Formatting
---
Every hand-authored .pw.html, .pw.sql, and .pw.dynamo source has one canonical form, produced by the system:tinybind formatter rather than by a Popcorn Web layout of its own.

```yaml
status: adopted; the repository is formatted and api:cli-fmt is implemented
priority: should
source: system:tinybind v0.3.1
scope: the concept:template-source-dialects sources; generated Go is already go/format's, per policy:generated-artifacts
why_not_ours:
  - the parser that would have to back a formatter is upstream's, and a second one would drift on every release
  - the only Popcorn Web input is the .pw suffix set, which the upstream pattern options already take
surfaces:
  cli: api:cli-fmt, for a terminal, a hook, and CI; still proposed
  editor_today: requirement:editor-formatting ships the embedded formatter, so an author can already canonicalize one buffer before this requirement is adopted
  editor: requirement:editor-formatting, whose delivery decision:formatter-delivery decides
  parity: both produce the same bytes for the same source and the same formatter version; a version difference between them is the risk decision:formatter-delivery bounds
not_in_the_generation_path:
  rule: formatting is never a precondition for api:cli-generate, and api:cli-generate never formats
  reason: a generator that rewrites its own input turns a build into an edit, and decision:explicit-generation-sources already fixed what a run may read
  dev_loop: api:cli-dev does not format either; a rewrite under a running watch would restart the loop from the loop's own write
ci:
  gate: pw fmt -l reports unformatted sources and writes nothing, so a pull request can require a formatted tree
  placement: alongside api:cli-check, which answers the adjacent question about generated drift and stays a separate command, per requirement:cli-generate-check-rename
adoption:
  one_time_cost: all 33 .pw sources in this repository change on first run, almost all of it the declaration body indent
  unblocked: system:tinybind v0.3.2 fixed the escape defect that made a repeated run unsafe, and every source now formats and settles
  remaining: the CI gate, which has no workflow to live in yet
  order:
    1: done; the pin moved to v0.3.5 and the page fixture was regenerated
    2: done; api:cli-fmt
    3: done; 32 sources reformatted, and the rule:template-grammar-scopes snapshot regenerated with them
    4: open; there is no Go CI workflow in this repository to add the gate to
  why_this_order: each step is separately reviewable, and step 3 is the only one that touches a hand-written source
verified:
  generation_equality: the page fixture's generated Go is byte-identical before and after its sources were formatted
  served_output: the helloworld document is byte-identical before and after formatting, measured through the example's own handler test
  idempotence: pw fmt --check is clean on every example project after one pw fmt run
  snapshot: the rule:template-grammar-scopes drift guard snapshot is regenerated in that same commit, because reformatted sources tokenize differently
acceptance:
  - formatting the repository twice changes nothing on the second run
  - generation output is byte-identical before and after formatting
  - a source that fails to parse is left untouched and reported with its path, line, and column
  - a comment survives formatting in all three dialects
  - pw fmt -l exits nonzero on an unformatted tree and zero on a formatted one
non_goals:
  - a Popcorn Web layout rule of its own; a disagreement with the upstream layout is reported upstream
  - formatting on generate, on save without consent, or on commit without an opt-in hook
  - formatting the .sql files under the migration directory, which are goose sources and not concept:template-source-dialects
```
