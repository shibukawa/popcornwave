# Density by page type

Not every page owes the reader the same thing. A tutorial chapter that dropped
into a table would stop teaching; a configuration reference written as narrative
would be unusable at the moment somebody actually opens it, which is with a
half-written TOML file on the other monitor. So the rule about prose is not "more
prose everywhere." It is: **the page carries reasoning in whatever form makes the
reasoning survive**, and for most pages that form is sentences.

Use this file to decide which standard a page is being held to before you audit
it or rewrite it.

## tutorial/

Four chapters, one project, `memoapp`, built in order. The reader types every
command and every file, so continuity is the whole contract: a code block in
chapter 3 has to be true for somebody who followed chapters 1 and 2 exactly.

Second person. Full files or clearly-marked diffs, never a fragment the reader
has to place. Every removal announced — when chapter 2 deletes the `homeInput`
the scaffold wrote, the block says so in a comment, and `check_docs.mjs --only=tutorial`
stays quiet because of that comment.

A chapter also carries a failure on purpose. Chapter 1 renames a template
parameter so the reader can watch the compiler catch it. That is not padding; a
reader who has seen the error message once recognises it later.

`references/exemplar-tutorial-getting-started.md` is the density target for this
directory and only for this directory.

## guides/

The five-part shape from SKILL.md, at the density of the exemplar. A guide is a
person deciding whether and how to use one feature, so it argues rather than
enumerates: two or three representative cases, each with the condition that
selects it, and a pointer to `reference/` for the rest.

Two sub-shapes live here, and confusing them produces most of the bad audits.

**Feature guides** — Discovered Routing, Async Rendering, Sessions, Fragments.
These are load-bearing prose. The reader has to leave understanding a model, not
a list of knobs. Hold these to the exemplar.

**Configuration-surface guides** — Compression, Security Headers. The feature is
one switch, or a fixed set of keys with fixed defaults. The interesting content
is the reasoning *around* the table: why the default is what it is, what the
switch costs, when to leave it alone. `guides/frontend/compression.md` opens with
a four-line TOML block, gives one table of what is covered and what is not, and
spends the rest of the page on the parts that need an argument — why there is no
gzip fallback, what streaming costs. That balance is correct. **Do not rewrite
these into narrative.** Flag them only when the table is doing work a sentence
should do, which is when a "because" went missing.

`references/exemplar-guide-compression.md` is the target for this sub-shape.

## reference/

Exhaustive by definition. Every key, every default, every environment-variable
name. Tables are the right form and a complete table is the point; a `reference/`
page that only covered the common cases would be broken.

Prose here is limited to the rules a table cannot express — how the three names
for one key are derived, which keys depend on which. Audit these for
completeness and accuracy, not for voice.

## pw/

One page per subcommand. Synopsis, flags, what it writes, what it refuses to do.
`pw/project/dev.md` is the shape: a short section per thing the command does at
startup, with the surprising behaviour spelled out.

These are close to reference pages and should not be padded toward guide density.
The one thing they owe a reader beyond the flag list is refusal behaviour — `pw
init` declining to write into a non-empty directory belongs on the page, because
that is what somebody hits.

## productivity/

Tooling around the framework rather than the framework itself: testing,
migrations, seed data, the dev identity provider. Guide-shaped, but the "when not
to" is usually about scope rather than alternatives — what the dev identity
provider is not safe for.

## start/

Orientation. `start/architecture.md` explains the request model and what a
minimal build excludes; `start/installation.md` gets the toolchain onto the
machine. Short, and they end by handing the reader to the tutorial.

## appendix/

`appendix/diagnostics.md` is a catalogue of every `pw doctor` finding, keyed by
code. Purely lookup: a reader arrives with `PW0113` in their terminal and leaves
as soon as they have the fix. One entry per code, same shape each time. Uniformity
is a feature here, and the prose rules do not apply.

## When a page resists classification

Ask what the reader had open when they arrived. Somebody mid-decision needs the
argument. Somebody mid-error needs the entry. Somebody mid-build needs the
complete table. Write for that, and the density settles itself.
