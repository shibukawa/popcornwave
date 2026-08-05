---
id: requirement:tutorial-capability-growth
type: requirement
title: Tutorial Capability Growth
---
Each tutorial chapter after the first opens by installing the capability it needs with api:cli-add, so the reader learns that a bootstrap answer is reversible by reversing one rather than by being told it is.

```yaml
audience: actor:application-developer
teaches: requirement:incremental-project-capabilities, on the project the reader has open
chapters:
  getting_started:
    init: the website-registered preset of requirement:init-presets, which already declines every capability the later chapters install
    why_that_preset: it is the smallest project the catalog names, so the chapter picks one list entry instead of answering five questions no, and the reader's first screen is the one every reader sees
    says: which capabilities were declined and that api:cli-add installs any of them later, which is the notice api:cli-init already prints
  forms:
    adds: tailwind
    for: ui:tutorial-memo-app styling, the first thing the reader sees rather than reads
  database:
    adds: database, answering sqlite at the engine step per requirement:database-engine-selection
    for: the memo table that replaces the in-memory slice
    other_engines: shown beside it under requirement:engine-parameterized-docs, since the chapter's DDL, query, and DSN are all dialect-specific
  login:
    adds: auth
    for: the login the chapter builds
  page_tree:
    adds: discovered
    for: requirement:tutorial-page-tree-chapter, which serves one route from a concept:page-tree and introduces the three update models over it
    coexists: decision:dual-router-coexistence, so the handlers of the earlier chapters keep serving
form:
  command: pw add <capability>, one line at the head of the chapter
  interaction: the decision:post-init-scaffold-wizard wizard runs and its review screen lists every file, so the chapter shows what the command is about to do before it does it
  no_flags: the engine is answered in the wizard rather than passed as a flag, because api:cli-add has no shortcut-flag parity
  after: the chapter names what appeared in the project, so the reader is not left comparing directory listings
rationale:
  - a reader who declined a capability at api:cli-init and then needs it is the ordinary case, and the tutorial is where they should have already seen the way out
  - the alternative is a project that starts with everything, which teaches that the questions did not matter
  - each chapter then owns one capability, one command, and one visible change
constraints:
  - the wizard is interactive, so the chapter shows the answers it gives rather than pretending the command is silent
  - api:cli-add refuses a capability the project already carries, so a reader repeating a chapter needs to be told that error is expected
  - auth depends on the database, so the login chapter cannot come before the database chapter
acceptance:
  - the capability list and the "Not included" output in getting-started match what the later chapters install
  - a reader who followed every chapter reaches the same project as one who answered yes to everything at api:cli-init
  - no chapter needs a capability the reader has not installed by the end of the chapter before it
non_goals:
  - teaching every capability in the catalog
  - a chapter for devbox or redis-valkey, which the tutorial never reaches for
```
