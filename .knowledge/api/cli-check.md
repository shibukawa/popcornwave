---
id: api:cli-check
type: api
title: pw check
---
pw check reports generated code that is stale or missing and writes nothing.

```yaml
usage: pw check
takes: no flag and no argument
compares: the concept:code-generation output the project's sources would produce, against the files on disk
writes: none
failure: generated content differs or is missing
exit:
  current: 0
  stale_or_missing: nonzero, listing the paths
scope:
  generated_go_only: dist/public and the generated stylesheet are excluded, because decision:generated-public-asset-version-control keeps both out of version control, so there is no committed content to compare against
  asymmetry: this verifies less than api:cli-generate writes; passing means the generated Go matches its sources, not that the tree compiles
  kind_agnostic: runs in an application and in a package project, since decision:committed-package-artifacts makes it the package release gate
both_halves_together: check mode plans the analysing half of concept:code-generation two_pass_ordering against the tree as it stands rather than against what a writing run would produce, because it writes nothing; a tree missing its generated files is stale, which is the answer this command exists to give
callers:
  - CI, as the gate policy:generated-artifacts requires, scaffolded by requirement:package-project-scaffold for a package
  - api:cli-doctor, which runs it before reporting the configuration sections it reads from generated metadata, per decision:host-side-diagnostic-analysis
  - requirement:editor-tasks, as the save-time task that writes nothing
  - the api:cli-lsp fallback of decision:language-server-in-pw-cli
neighbours:
  api:cli-fmt: --check answers template formatting
  api:cli-doctor: answers configuration and wiring
  division: this command answers generated drift alone, and none of the three runs the others
naming:
  was: a --check flag on api:cli-generate, until requirement:cli-generate-check-rename
  why_a_command: the gate every CI runs was a flag on a command whose default writes, so a reader scanning the command list did not find it; and the command it hung from now writes an asset tree this one does not check
reporting: policy:cli-progress-reporting
```
