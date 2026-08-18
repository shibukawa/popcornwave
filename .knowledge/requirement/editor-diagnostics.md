---
id: requirement:editor-diagnostics
type: requirement
title: In-Editor Diagnostics
---
A problem the developer would otherwise find at api:cli-generate or api:cli-dev appears in the editor at the position that causes it, with the same identifier and the same wording.

```yaml
status: proposed
stage: 2 of vision:editor-support
server: requirement:pw-language-server
one_message_rule: decision:shared-check-catalog, extended to a third runner
sources:
  syntax:
    from: the concept:template-source-dialects parsers
    when: on every change, without saving
    examples: an unterminated body, an unknown root declaration, a malformed annotation, an unbalanced control form
  generation:
    from: the same checks api:cli-generate stops on
    when: on save
    examples: an unknown external function, an output type the root keyword does not allow, an SQL result contract the statement cannot satisfy, a dynamo attribute absent from the type's tags
  project:
    from: the decision:shared-check-catalog entries whose inputs are project files
    examples: a source outside its generate purpose, a generate entry naming a missing directory, an unknown popcornweb.toml key, a dynamo declaration naming a table decision:dynamodb-table-registry does not know
    position: the popcornweb.toml line for a configuration finding, the file for a placement finding
severity:
  error: what would stop api:cli-generate
  warning: what api:cli-generate warns about, such as a source outside its purpose
  hint: an api:cli-doctor advisory, reported only when its inputs exist
skipped:
  behavior: a check whose inputs the editor cannot build is not run and not reported as passing
  visibility: listed on request rather than as a diagnostic, because a silent skip reads as a clean file
staleness:
  rule: a diagnostic from a save-time source is cleared when its document changes and republished after the next analysis, so a fixed error does not linger
non_goals:
  - a diagnostic the CLI cannot also produce
  - running api:cli-generate to obtain a diagnostic, which would write output; the fallback shell-out of decision:language-server-in-pw-cli uses api:cli-check and is opt-in
  - Go diagnostics, which gopls owns, including those in a generated *_pw_gen.go
acceptance:
  - an unterminated component body is reported before the file is saved
  - a renamed dynamo tag makes its .pw.dynamo declaration report the same error api:cli-generate reports, at the attribute
  - a .pw.sql moved outside every generate.queries entry reports the warning api:cli-generate warns with, naming the key
  - the identifier shown in the editor is the identifier api:cli-doctor prints for the same condition
  - closing and reopening a file reproduces the same diagnostic set
```
