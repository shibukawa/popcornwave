---
id: requirement:tutorial-continuity
type: requirement
title: Tutorial Continuity
---
The tutorial is read in order and typed along with, so a reader never has to reconstruct which lines changed since the last chapter, and never meets a framework symbol the page has not explained.

```yaml
audience: actor:application-developer
surface: the tutorial chapters, in every language the site publishes
running_example: one project carried from api:cli-init through forms, a database, and a login, so almost every code block after the first is an edit of a block the reader already typed
rendered_result: ui:tutorial-memo-app, which owns what that project looks like at each step
capability_growth: requirement:tutorial-capability-growth, which owns what each chapter installs before it starts editing
file_identity:
  rule: a block the reader types into a file names that file on its first line, as a comment in that language
  form: the path from the project root, the way the chapter's own file tree writes it
  ordering: the file name comes first, and the note about which earlier block this replaces comes under it
  prose_is_not_enough: a sentence above the block is read once, and the reader who scrolls back to copy the code lands inside the block rather than above it
  applies_to: Go, template, SQL, and TOML blocks alike; a TOML fragment is the case with the most files it could plausibly belong to
  excludes: terminal transcripts, command lines, and response bodies, which go into no file
  known_gap: five blocks across the Japanese chapters name their file today, and the rest rely on the sentence above them or on nothing
  worked_instance: the forms-chapter block that follows "the handler decides what to do with the failure", which names the block it replaces but not the file it replaces it in
changed_code:
  rule: a block that modifies earlier code marks what changed, in the code, as a comment
  form: source comments, which is what the tutorial already uses to explain a line in place
  marks:
    changed: the line or function that differs, naming what it replaced
    new: the import, function, or field that was not there before
    unchanged: worth marking only in a full-file listing, where silence reads as "this changed too"
  partial_block: a block showing part of a file says which earlier block it replaces
  reason: a full-file listing is the honest way to show a file, and it is also the form that hides a three-line edit inside forty lines
  not_a_diff: prose still carries the why; the comments carry only the where
first_appearance:
  rule: a framework symbol is explained where the tutorial first shows it, not where a reference page defines it
  depth: what it is and why it is not the standard-library thing the reader expected; the reference page carries the rest
  worked_example: pw.NewServeMux in the getting-started handler section, explained as the net/http.ServeMux type alias it is on host Go and the compatible implementation it becomes under TinyGo, which is the depth this rule means
follows_the_product:
  rule: a chapter describing a command's behavior tracks the concept that owns it
  pending: the getting-started claim that nothing is asked when a project name is given, which decision:interactive-project-bootstrap reverses
acceptance:
  - every code block destined for a file names that file inside the block
  - every code block that edits an earlier one carries a mark on each changed and each added region
  - a reader who typed only what the tutorial showed has a compiling project at the end of every chapter
  - no framework symbol appears in a code block before the page has said what it is
  - the language versions carry the same marks and the same explanations
non_goals:
  - rendered diffs or line highlighting in place of comments, which a reader cannot copy along with the code
  - explaining a symbol again in every chapter that uses it
  - a reference-page level of depth inside the tutorial
```
