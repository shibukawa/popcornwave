---
id: requirement:dev-reload-summary
type: requirement
title: Development Reload Summary
---
api:cli-dev captures the startup summary its application process printed and reports the next one as a difference from it, so a rebuild costs one line when nothing changed and the changed rows when something did.

```yaml
audience: actor:application-developer
scope: api:cli-dev only; pw itself carries no reload behavior and an api:cli-build artifact is untouched
problem:
  repetition: every watched change starts a new process, and a new process emits policy:startup-summary in full, so the banner and every resolved key are reprinted for a rebuild that changed one template
  cost: the summary is taller than the terminal region the loop exists to fill with application and service output, so what the developer was reading scrolls away behind output saying nothing new
  not_the_summary_itself: the first one answers what the run resolved to and is why the format exists; only the repetition is the problem
first_boot:
  behavior: the captured block is passed through unchanged, so a session still opens with policy:startup-summary as it stands
  reason: nothing has been read yet, so there is no shorter answer that is still true
reload:
  unchanged: the single line "reloaded"
  changed: the changed rows only, in the same tree, with every unchanged sibling and every section holding none of them dropped
  ancestors: a changed row keeps its section path, so a key is still read where it lives
  banner: never reprinted, on either path
  destination: the stream the block would have gone to
rows:
  changed: old and new in the value column, as "5s → 10s"
  added: marked added, with the value it resolved to
  removed: marked removed, with the value it last had
  why_both: a key gated on another leaves the report entirely when its gate is turned off, so appearing and disappearing is the ordinary way a section changes rather than a rare case
  place_only: a key whose value survived but whose winning place did not is a change, named on both sides, because policy:startup-summary reports the place for the same reason it reports the value
  compared_beside_the_keys:
    what: the accepted listening address, data:runtime-environment, the resolved config path, and the framework version, so a decision:development-port-shift onto another port is one line rather than silence
    where: their own lines above the tree, since none of them is a configuration key and the tree is what the summary calls configuration
  volatile: the start time is excluded, since comparing it would make every reload a change
  secrets:
    rule: not compared beyond what policy:log-emission left visible, so a changed credential behind the same mask reports as unchanged
    accepted: deliberately; the alternative is carrying a digest of the value before redaction, and a development convenience is not worth a second representation of a secret
    dsn: rule:dsn-redaction keeps a public half, so a moved host or database still shows
mechanism:
  who: the loop, from the application's own output, which is why nothing about reloads reaches pw
  capture: the application process gets its own stdout and stderr, and everything it writes is passed through unchanged except the block policy:startup-summary emitted
  block: recognized from the first banner line to the listening line
  incomplete: a process that exited or wrote something else before the block completed has what was held flushed verbatim, so a crash mid-boot costs nothing and hides nothing
  latency: only the block is held back, and only until it completes or the process writes past it
  previous: kept in the loop process for the run, so nothing is written to disk and nothing survives into the next session
  parse:
    shape: each row splits into label, value, and place; the label carries the key path in its indentation
    safe: a tree label never contains two consecutive spaces, so the first run of two or more is where the value starts
    fallback: a block that does not parse is printed verbatim, which is the behavior it replaces
    cost: the internal/pwtree layout becomes something read by a program as well as a person, so a layout change has to keep the diff working; this is the price of the application not knowing that reloads exist
  piped_stderr:
    effect: a pipe is not a character device, so the application resolves observability.boot_log auto to record and renders without color
    format: the loop pins the child to tree through the generated environment binding for observability.boot_log, and leaves an explicit setting alone whether it came from the development configuration, read best effort, or from the developer's own shell
    color: the child honors CLICOLOR_FORCE, which the loop sets when its own stderr is a terminal; NO_COLOR still outranks it, because that one is the developer speaking rather than a caller
    home: package internal/bootblock, which also owns the banner art, so the summary is drawn and read back in one place rather than by two that can drift
formats:
  tree: as above
  record: unchanged, one record per process, because a collector deduplicates and a reader of records is not watching a terminal
  off: unchanged, nothing, not even the reloaded line
non_goals:
  - a timestamp, an elapsed time, or a change count on the reloaded line
  - naming what triggered the reload, which policy:cli-progress-reporting already reports as the phase
  - diffing anything but the reported configuration, so not the routes, not the schema, and not the generated files
  - any behavior outside api:cli-dev, since a process started by hand has nothing to compare against
acceptance:
  - the first application start of a session prints what it prints today, color included
  - a rebuild that changed only a template prints "reloaded" and nothing else
  - editing html.bot_async_timeout prints the html section carrying that one row with its old and new value, and no other row
  - turning html.bot_detection off prints the keys it gated as removed, not a full tree
  - a configured port already taken prints the accepted address as a changed row
  - an application that panics during startup loses none of what it wrote
  - observability.boot_log record and off are unaffected by all of it
```
