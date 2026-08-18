---
id: requirement:cli-generate-check-rename
type: requirement
title: pw generate And pw check Renaming
---
The command that leaves a compilable tree is named pw generate and the command that verifies generated drift is named pw check, so the name every caller guesses first is the one that leaves nothing missing.

```yaml
problem:
  trap: generate is the name a reader guesses for "make a tree the compiler can read", and it named the command that wrote only the generated Go; dist/public was absent, so the compiler failed on a go:embed directive over a directory nothing built, and a requirement:tailwind-css-integration project rendered unstyled instead of failing at all
  evidence: the prepare documentation page carried a section titled "pw generate is not enough" and the api:cli-build page repeats the same split; a trap needing a section on two pages is a naming defect rather than a documentation gap
  prepare_is_unguessable: nothing in the word prepare names any of its four steps, so a caller who has not read the page cannot tell it from generate, and the one that fails is the one they reach for
  check_is_hidden: generated drift is the gate every CI runs, and it lives as a flag on a command whose default writes, so a reader scanning the command list does not find it
naming:
  pw_generate: every host step the prepare command ran, stopping before the compiler
  pw_check: what the --check flag did, unchanged in what it compares, as api:cli-check
  code_only_flag: --code-only on pw generate runs the code-generation step alone, which is what the narrower generate did
  rejected_no_assets: --no-assets names what is skipped; the reader of a CI line wants what is produced
  rejected_alias: keeping prepare as an alias keeps two names for one job, which is the defect being removed
concept_split:
  api:cli-generate: the command — its steps, its flags, its package-kind behavior, its callers; the prepare body moved here
  api:cli-check: taken from the api:cli-generate check_mode block
  concept:code-generation: the contract moved out of api:cli-generate — discovery scope, artifacts, two_pass_ordering, unparsable_source — because four callers share it: the first step of the command, api:cli-check, api:cli-dev, and api:cli-build through the command
  deleted: the api concept for prepare, whose id no longer resolves and is not linked from here for that reason
  reference_rule: an existing api:cli-generate reference stays valid and is left alone, because the renamed command is a superset of the old one — everything said about what it reads, warns about, or writes is still true of it; only a reference naming --check, check mode, or prepare is wrong and gets edited
check_scope:
  compares: generated Go only, unchanged
  excluded: dist/public and the generated stylesheet, because decision:generated-public-asset-version-control keeps both out of version control, so no committed content exists to compare against
  asymmetry: pw check verifies less than pw generate writes; its page states this rather than leaving a reader to infer that a passing check means a compilable tree
  neighbours: api:cli-fmt --check keeps template formatting and api:cli-doctor keeps configuration and wiring, so pw check answers generated drift alone
  supersedes: the system:pw-cli initial_exclusions entry ruling out a standalone pw check, which held only while --check was a flag on the command that writes
code_only_flag:
  runs: the code-generation step alone, writing no asset tree and no stylesheet
  keeps: the development-only import rejection, because a flag must not be the way past a security gate, and its cost is a dependency-graph load rather than a compile
  for: the requirement:editor-tasks generate command, and a developer who wants generated Go without waiting for a minified stylesheet
  not_for: a tree handed to a compiler, which is the unflagged command
package_projects:
  problem: the prepare command was refused in a package project, while api:cli-generate is the command a concept:component-package generates and releases with, so the renamed command must run there
  generate: runs, and reaches only the code-generation step — there is no project.main whose imports could be rejected, no asset tree, and no stylesheet; the same result --code-only gives, selected by project.kind rather than by flag
  check: runs, and stays the package release gate of decision:committed-package-artifacts
  refusal: the package refusal stays on api:cli-build and leaves the renamed command
compatibility:
  aliases: none; pw prepare and the --check flag are removed in the same change
  breakage: a scaffolded CI workflow and a Dockerfile.tinygo in an existing project stop working, accepted because system:pw-cli publishes no compatibility window for command names
  retired_names: no diagnostic mentions them; pw prepare meets the generic unknown command error and --check the generic unknown argument error, the same as any name the CLI never had
  no_residue: nothing in the dispatch, the flag parsing, or the help text names a removed command, so there is no entry a later reader has to decide whether to keep
affected_surfaces:
  dispatch: the internal/pwcli command switch and its summary table, which are the two places a name lives; the prepare entry became generate, the check half of the generate entry became check, and check is listed beside generate rather than with the diagnostics because it answers a question about generate's output
  usage_strings: the build usage string named prepare, and the generate usage string named --check
  flags: --target stays api:cli-build only and its rejection moved to generate; --code-only is refused on build, and refused alongside --debug
  scaffolds: the package CI workflow api:cli-init writes, and the Dockerfile.tinygo recipe of requirement:container-image-scaffold, which ran the retired names
  docs:
    renamed: the pw prepare page became the pw generate page, keeping the what-it-reads and what-it-writes sections of the old generate page so the anchor api:cli-new links to still resolves; the check half became a pw check page
    redirects: the retired prepare URL and its Japanese counterpart, in the website redirect table
    deleted: the "pw generate is not enough" section, whose reason to exist is gone
    mostly_unchanged: most pages naming pw generate need no edit, because the new command is a superset and an instruction to run it stays correct; only a page describing what it does not write is wrong
    parity: every English edit has its Japanese counterpart
  skills: the repository Popcorn Web skill, and the example READMEs that ran prepare before a TinyGo compile
  catalog: every concept naming the retired flag or the retired command, per the concept_split reference_rule
acceptance:
  - pw generate in an application project leaves a tree host go build compiles from a clean clone, with no second command to remember
  - pw generate --code-only writes generated Go and neither dist/public nor a stylesheet
  - pw generate in a package project succeeds and writes generated Go only
  - pw check writes nothing and fails on a stale or missing generated file, in an application and in a package project
  - pw prepare and pw generate --check each fail as an unknown command and an unknown argument
  - help lists generate and check, and no longer lists prepare or --check
  - no documentation page tells a reader that pw generate is not enough
  - the website documentation checks report no broken link and no English or Japanese drift
non_goals:
  - folding api:cli-fmt --check or api:cli-doctor into pw check
  - changing what the code-generation step reads or writes
  - changing api:cli-build, which stays defined as api:cli-generate plus the compiler
```
