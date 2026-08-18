---
id: decision:canonical-popcornweb-skill-source
type: decision
title: Canonical Popcorn Web Skill Source
---
Author the bundled framework skill once at repository-root `skills/popcornweb-skill/`; api:cli-init embeds that same tree and copies it to one selected agent discovery path.

```yaml
problem:
  - the repository source and api:cli-init output appear at different paths, which can look like two maintained skill trees
decision:
  canonical: skills/popcornweb-skill/
  rename: none
  embed: the Go skills package embeds the canonical directory directly
  generated_name: popcornweb
  generated_destinations:
    claude: .claude/skills/popcornweb/
    agents: .agents/skills/popcornweb/
  none: api:cli-init writes no skill when the developer declines it
reasoning:
  - repository-root skills/ remains the only authored source and the installable repository artifact
  - api:cli-init needs an embedded copy so project creation remains offline
  - agent-specific destinations are discovery adapters, not independently maintained sources
  - popcornweb-skill is the repository package name while popcornweb is the installed skill name; the differing names do not create a second source
rules:
  - never maintain a second template or pw-init-only skill tree
  - tests compare every generated skill file byte-for-byte with the embedded canonical source
  - references use paths relative to SKILL.md so the same tree works at every destination
  - documentation calls generated trees copies and points updates to skills/popcornweb-skill/
  - a change to requirement:agent-log-analysis-skill lands in the canonical tree and therefore in every later scaffold
migration:
  repository: no directory migration; add new guidance to the existing canonical tree
  existing_projects: no automatic rewrite; their checked-in copy remains valid and may be refreshed explicitly from the canonical tree
  cli_compatibility: keep --skills=claude, agents, or none and its defaults unchanged
rejected:
  root_only_output:
    shape: generate only skills/popcornweb/ in an application project
    reason: a generic root skills directory is not a common automatic discovery path, so the skill can exist without being offered to the agent
  symlink_adapters:
    shape: generate the root tree plus symlinks from every agent directory
    reason: it writes more paths, complicates Windows and archive handling, and still cannot remove agent-specific discovery entries
  cosmetic_rename:
    shape: rename skills/popcornweb-skill to skills/popcornweb only to match installed directory names
    reason: it changes embed and documentation paths without removing a maintained copy or changing discovery behavior
  duplicated_templates:
    shape: author one root skill and one separate pw init skill
    reason: content and query guidance drift silently
```

```yaml
acceptance:
  - the repository contains one authored Popcorn Web SKILL.md tree at skills/popcornweb-skill
  - the embedded filesystem reads that same tree
  - each non-none api:cli-init answer writes exactly one generated copy
  - generated copies include requirement:agent-log-analysis-skill guidance and every referenced file
  - no public flag or existing generated-project path changes
```
