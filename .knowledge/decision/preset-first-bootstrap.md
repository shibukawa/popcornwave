---
id: decision:preset-first-bootstrap
type: decision
title: Preset First, Questions Behind It
---
api:cli-init opens on the requirement:init-presets list rather than on its first question, and a preset answers every capability question at once, leaving the project name as the only thing still asked.

```yaml
status: accepted
decided: user 2026-08-05
supersedes: the decision:interactive-project-bootstrap ordering, under which every run walked the whole question list from the project name down
question: how a bootstrap stays explainable once the answers outnumber what a first project can decide
answer:
  step_one: a single-select of the presets, Manual last
  preset_path: project name, then the review; nothing else is asked
  manual_path: decision:navigable-answer-hub, which is the review screen with every row editable
  review_is_shared: a preset's review and the Manual hub are one screen, seeded differently
why_the_review_is_the_hub:
  observation: the review screen already lists every question and its answer, which is the shape a preset needs to explain itself and the shape a back-and-forth editor needs to navigate
  effect: a preset is not a path that hides the questions; it is a path that answers them and shows the answers in the same place Manual edits them
  consequence: enter on a review row opens that question and returns to the review, so a preset is a starting point rather than a commitment
  cost: one screen instead of two, and the linear stepper stays for the commands that still want it
name_argument: unchanged; it seeds the project name step, and the preset list is still the first screen
shortcut_flags:
  preset: --preset=<name>, which answers what the wizard would have asked
  conflict: --preset with any capability flag is refused before anything is written, because both answer the same questions and neither is obviously the winner
  yes: --yes without --preset keeps its current meaning, the flags and the defaults
  no_terminal: unchanged, per decision:interactive-project-bootstrap
project_kind:
  rule: a preset may set project.kind, and one that does removes the questions that do not apply to the kind
  instance: the package preset, per requirement:package-project-scaffold, which reaches the api:cli-package scaffold rather than answering the application questions
  mechanism: the same conditional-step rule that already hides a question the answers do not reach, read against the kind
  flag: --kind stays the scriptable spelling, and --preset=package is the discoverable one
what_stays:
  question_list: every question survives; a preset supplies answers to it rather than replacing it
  conditional_steps: unchanged, and a preset that answers a step whose condition does not hold has that answer dropped the same way a flag does
  extension_cost: a new option is still one option field, one flag, one step, and one scaffold branch, plus a line in each preset that has an opinion about it
  linear_stepper: kept for api:cli-add and api:cli-new, per decision:post-init-scaffold-wizard, which review a plan rather than a set of answers
rationale:
  - the ten questions were each defensible and the sequence was not, because most of the combinations they can express are ones nobody wants
  - a named shape is documentable, recommendable, and comparable; a combination of ten answers is none of those
  - showing the answers a preset gave is what keeps it from being a black box, and it costs the screen that already existed
rejected_fewer_questions:
  form: drop or merge questions until the list is short
  why_not: every question the list carries decides something a project cannot change cheaply afterwards, and merging them makes the answers less visible rather than fewer
rejected_preset_only:
  form: presets replace the questions
  why_not: the combinations outside the four are ordinary, and requirement:incremental-project-capabilities would become the only way to reach them, one reviewed command at a time
non_goals:
  - remembering the last run's answers
  - a user-defined preset file
```
