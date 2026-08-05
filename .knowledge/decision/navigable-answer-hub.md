---
id: decision:navigable-answer-hub
type: decision
title: Manual Bootstrap Edits an Answer Hub
---
The Manual preset of requirement:init-presets navigates a list of every question and its current answer, opening one question at a time and returning to the list, instead of walking the questions in order from first to last.

```yaml
status: accepted
decided: user 2026-08-05
scope: api:cli-init only; api:cli-add and api:cli-new keep the linear stepper of decision:post-init-scaffold-wizard
prior_art: vue-cli, where a preset list and a feature editor sit in front of the same option set
question: how an operator revisits the third answer after the eighth question changed what it should be
answer:
  screen: one hub listing every applicable question with its current answer, plus a create row
  navigate: up and down over the rows, digits to jump
  open: enter on a row shows that question, which is the existing step screen unchanged
  return: accepting or leaving a question returns to the hub, not to the next question
  create: enter on the create row writes the project
  cancel: ctrl+c, unchanged
same_screen_as_review:
  fact: the hub is the decision:preset-first-bootstrap review screen with its rows made editable
  effect: Manual is the hub opened on the defaults, and a preset is the hub opened on the preset's answers
  reason: two screens that list every question and its answer would drift, and the review already had the harder half
conditional_rows:
  rule: a row appears when its step applies to the current answers, per the decision:interactive-project-bootstrap conditional-step rule
  recompute: after every accepted answer, because a step's condition reads the answers before it
  disappearing_row: its answer is dropped, unchanged from the linear wizard
  appearing_row: shows the default it would have been asked with, and is marked as never answered
  cursor: stays on the row it was on when one appears or disappears above it, so an answer does not move the selection
answered_marking:
  rule: a row records whether the operator opened it or the value is a default
  shown: an unopened row reads as the default rather than as a decision
  why: a hub with no first-to-last order has no other way to say which answers were considered
ordering:
  rule: the rows keep the linear order of decision:interactive-project-bootstrap
  reason: the dependent-follows-dependant ordering is what makes the list readable top to bottom, and a hub is read that way even when it is not walked that way
  not_grouped: no sections, because every grouping puts the router and the store questions in different places and they are one decision
first_run_concern:
  problem: a hub shows every question at once, which is what a first project was not able to answer
  answer: a first project takes a preset and never opens the hub with nothing decided; Manual is the path for someone who already knows what they are choosing
implementation:
  reuse: the wizardStep list, the choice and text steps, the conditional wrapper, and the theme are unchanged
  new: a hub model that owns the cursor, the answered set, and the open step, replacing the index walk for this command
  library: github.com/charmbracelet/bubbletea, host-only per decision:host-tools-target-runtime
rationale:
  - the answers are dependent, so the run that most needs to go back is the run that understood the question only after seeing a later one
  - esc-to-previous was a one-step undo, and reaching the third answer from the ninth meant six presses and six re-accepts
  - a hub makes the whole answer set visible, which is what a reviewer of a preset wants as well
rejected_esc_history:
  form: keep the stepper and let esc walk further back
  why_not: it is the same six presses with a shorter name, and it still shows one question at a time
non_goals:
  - editing an answer after the project is written
  - a hub for api:cli-add or api:cli-new, whose review approves a file plan rather than an answer set
  - saving a hub session as a preset
```
