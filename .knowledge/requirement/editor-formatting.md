---
id: requirement:editor-formatting
type: requirement
title: Editor Formatting
---
The editor formats a .pw source into the canonical form of requirement:template-formatting, on request from the first install and on save only once the formatter is safe to run unattended.

```yaml
status: implemented at tools/vscode, version 0.3.0, embedded path only
stage: 1.5 of vision:editor-support; it needs no project model, so it does not wait for requirement:pw-language-server
not_yet_implemented:
  delegated_path: api:cli-fmt now exists, but the extension does not call it yet; it always uses the embedded module and says so once per session
  effect: the decision:formatter-delivery version-skew risk is live rather than mitigated, and the session line is what makes it visible
platform: system:vscode
delivery: decision:formatter-delivery
provides:
  - a DocumentFormattingEditProvider for pw-html, pw-sql, and pw-dynamo
  - a DocumentRangeFormattingEditProvider only if the formatter gains a range entry; a whole-document reformat presented as a range edit would surprise
  - the Format Document command, and nothing bound to save by default
edit_shape:
  applied_as: one full-document replacement, because the formatter returns bytes rather than a diff
  cursor: the editor's own edit reconciliation keeps the cursor; the extension computes no mapping of its own
  unchanged: when the formatted bytes equal the buffer, no edit is returned at all, so an undo stack gains nothing
guard:
  rule: never apply an edit that has not been checked to settle, because a formatter defect edits the user's source
  owner: system:tinybind from v0.3.2, whose Source and SourceAs format twice and error rather than return a differing result
  why_not_here:
    was: the extension carried the check itself, because v0.3.1 had none and grew a brace pair per pass on a literal brace run in a script or style body
    now: the same check upstream sits closer to the AST, so repeating it costs a second round trip to defend only against that guard being broken
    kept_instead: a version floor, asserted by a test, so a downgrade below v0.3.2 fails rather than silently removing the protection
  on_failure: report once naming the line, return no edit, and leave the buffer byte for byte
  delegated_path_note: a project pinning an older tinybind has no guard, so the api:cli-fmt path must either require v0.3.2 or restore the check; decision:formatter-delivery carries how that path detects and selects
format_on_save:
  default: off, because an extension enabling it for the user is presumptuous rather than because it is unsafe
  safe_since: v0.3.2; every source in this repository formats and settles, which was the stated condition for withdrawing the recommendation against it
  enabling: the ordinary editor.formatOnSave setting, which a user or a workspace sets per language; the extension contributes no setting of its own for it
diagnostics:
  parse_failure: reported as a notification rather than as a diagnostic, because requirement:editor-diagnostics owns the Problems view and does not exist yet
  later: once the language server ships, a parse failure is a diagnostic and this notification is removed rather than duplicated
non_goals:
  - formatting a generated *_pw_gen.go, which gopls and go/format own
  - formatting the embedded HTML or SQL with a third-party formatter, which would not know the template forms
  - an organize-imports or sort-attributes action, which requirement:template-formatting rules out at the source
acceptance:
  - Format Document on each of the repository's own sources produces the same bytes api:cli-fmt produces for that source
  - a source with a syntax error is left untouched and reported once
  - a literal brace run in a script or style body survives formatting unchanged and settles on the second pass
  - formatting works in a window with no folder open and no pw on PATH
  - formatting an already canonical buffer returns no edit
  - the path that produced the result, embedded or delegated, is reported once per session
```
