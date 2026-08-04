---
id: system:vscode
type: system
title: Visual Studio Code
---
Visual Studio Code is the first editor Popcorn Wave targets; its extension contract decides what vision:editor-support can deliver without a server and what needs one.

```yaml
extension_points_used:
  contributes.languages: language id, file extensions, and configuration file, per requirement:editor-language-registration
  contributes.grammars: TextMate grammar plus embeddedLanguages, per requirement:template-syntax-highlighting
  contributes.configurationDefaults: per-language editor defaults
  contributes.commands and contributes.taskDefinitions: requirement:editor-tasks
  contributes.problemMatchers: maps api:cli-generate and api:cli-doctor output into the Problems view
  languageClient: requirement:pw-language-server, only from stage 2
grammar_engine:
  implementation: Oniguruma regular expressions over a line-oriented scope stack
  can: nested begin/end rules, embedded grammars by scope name, injection into another grammar
  cannot: count, balance braces reliably, resolve types, or see another file
  consequence: decision:textmate-grammar-first accepts approximate highlighting; anything needing resolution waits for the server
embedded_language_support:
  mechanism: a grammar region tagged with a scope whose language is declared in embeddedLanguages
  effect: bracket matching, comment toggling, and snippets follow the embedded language inside the region
  limit: one region has one language, so an HTML body containing a template expression must reopen the outer scope for that expression
workspace_trust:
  fact: an untrusted window may load an extension in restricted mode and must not run workspace binaries
  consequence: policy:editor-tool-execution
distribution:
  registries: Visual Studio Marketplace and Open VSX
  package: vsix, built by vsce or ovsx
  consumer: requirement:extension-distribution
already_scaffolded:
  file: the concept:project-layout .vscode/settings.json that hides **/*_pw_gen.go
  relation: the extension supersedes the need for that hand-written entry but does not remove it
non_targets_for_now:
  - JetBrains and Neovim, which decision:language-server-in-pw-cli keeps reachable without a second analysis implementation
```
