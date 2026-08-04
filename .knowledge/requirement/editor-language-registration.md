---
id: requirement:editor-language-registration
type: requirement
title: Editor Language Registration
---
Each concept:template-source-dialects suffix is a distinct editor language with its own identity, icon, comment syntax, and bracket rules, so the editor stops treating the file as plain text before any grammar runs.

```yaml
status: implemented at tools/vscode, version 0.1.0
stage: 1 of vision:editor-support
platform: system:vscode
languages:
  pw-html:
    extensions: [".pw.html"]
    aliases: ["Popcorn Wave HTML"]
    reason: a plain html association would apply an HTML formatter to a file whose header is not HTML
  pw-sql:
    extensions: [".pw.sql"]
    aliases: ["Popcorn Wave SQL"]
    reason: a plain sql association would send the file to a SQL client extension as an executable script
  pw-dynamo:
    extensions: [".pw.dynamo"]
    aliases: ["Popcorn Wave DynamoDB"]
language_configuration:
  comments: the header comment syntax; the body inherits the embedded language through the rule:template-grammar-scopes embeddedLanguages map
  brackets_and_autoclosing: braces, parentheses, and angle brackets in pw-html; braces and parentheses elsewhere
  autoclosing_exclusion: no auto-close inside a string or comment scope
  indentation: increase after an opening declaration brace and after an opening control form
configuration_defaults:
  - per-language defaults only, so a workspace-wide editor setting is untouched
  - trim trailing whitespace is left to the user, because an HTML body may depend on it
project_awareness:
  reads: data:project-config only to answer whether a suffix sits inside its generate purpose, and never to decide the language
  absent_config: every feature of this stage works without popcornwave.toml
generated_files:
  fact: concept:project-layout scaffolds a .vscode/settings.json hiding **/*_pw_gen.go
  behavior: the extension does not remove or duplicate that entry, and does not hide files on its own
  proposed: api:cli-init also writes .vscode/extensions.json recommending the extension, deferred until requirement:extension-distribution publishes an identifier
acceptance:
  - opening a .pw.sql selects pw-sql and not sql
  - opening a .pw.html selects pw-html and not html
  - the language selector shows all three by their aliases
  - toggling a comment on a header line uses the header comment syntax, and on an SQL body line uses the SQL one
  - no setting outside the three language sections is written
```
