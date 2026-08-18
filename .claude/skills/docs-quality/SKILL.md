---
name: docs-quality
description: Write, rewrite, review, or audit any page of the Popcorn Web documentation site under website/src/content/docs/ (Astro Starlight, English + Japanese). Use this whenever documentation is created or changed — "write the docs", "add a guide", "document this feature", "update the docs", "review this page", "the docs are stale", "translate this page" — and ALSO immediately after implementing or changing a framework feature that users will need documented, even when the request only says "and update the docs too". Carries the house style (prose over bullets, guides show two or three cases rather than every option, every guide says when not to use the feature, recommend rather than survey), the fixed terminology (registered/discovered routing, .pw.html as a typed template language, _pw_gen.go as build output, pw.ServeMux as a type alias), and check_docs.mjs for broken links, retired URLs, frontmatter, English/Japanese parity, Tailwind leakage, tutorial code continuity, and sidebar drift.
---

# Popcorn Web documentation

The site is Astro Starlight. Pages are Markdown under
`website/src/content/docs/`, English at the root and Japanese under `ja/`, and
the sidebar is declared once in `website/astro.config.mjs` — a page appears by
dropping a file into a directory the config already autogenerates, so most work
here never touches the config at all.

Paths below are relative to the repository root.

Two things carry this skill. `check_docs.mjs` decides everything a machine can
decide, and it runs in under a second on the whole site. The rest of this file
covers what it cannot: whether a page is worth reading.

**Start with the script, in either mode.** It will hand you the mechanical
findings so your own attention goes to prose.

```bash
node .claude/skills/docs-quality/check_docs.mjs
```

Scope it while you work on one area, and re-run it unscoped before you finish:

```bash
node .claude/skills/docs-quality/check_docs.mjs --path=guides/frontend
```

Exit status is 1 when any `error` finding is present, so it drops into a
pre-commit hook or CI unchanged. `--json` gives structured output; `--only=links,parity`
restricts to named checks (`links`, `frontmatter`, `parity`, `tailwind`,
`tutorial`, `sidebar`, `terms`, `shape`).

The checker's own tests are the fastest way to confirm it still works after you
edit it:

```bash
node .claude/skills/docs-quality/tests/run_tests.mjs
```

## Mode 1 — writing or rewriting a page

### Decide what kind of page this is first

A guide, a reference page, and a `pw` subcommand page owe the reader different
things, and holding all of them to one density is the most common way to make
this site worse. `references/page-types.md` settles that question per directory.
Read it before you write, especially when the page you are touching is a
configuration surface — those are correctly table-forward and must not be
rewritten into narrative.

### The shape of a guide

Five parts, in this order.

1. **What problem this solves.** Open on the reader's situation, not on the
   feature's name. The compression guide opens with the switch and then explains
   why it is off by default; the discovered-routing guide opens with three lines
   of repeated registration code that the router removes.
2. **A complete example that runs when pasted.** Complete means it compiles —
   imports present, `package` clause present, no `// ...` standing in for
   something the reader would have to invent.
3. **The decisions, with the condition that selects each one.** Two or three
   cases. Not the full matrix.
4. **The pitfalls.** What looks like it should work and does not.
5. **A link into `reference/`** for the exhaustive list.

### Prose carries the reasoning

A bullet list and a table both drop the connectives, and the connectives are
usually the content. When you find yourself writing a row, ask whether the row
still says *because*. If it does not, it goes back into a sentence.

This is not a ban on tables. A table is right when the items are genuinely
coordinate — a matrix of what is compressed against what is not, a flag list, a
symptom-to-fix lookup. It is wrong when it flattens an argument into cells and
leaves the reader to reconstruct why any row is true.

The corresponding failure in the other direction is a page that lists every
option a feature has. Exhaustiveness belongs to `reference/`. A guide that
enumerates has taken on a maintenance burden it cannot carry and buried its
recommendation in the middle of a table.

### Recommend

Where the reader has to choose, say which one they should take and why, then
name the condition that changes the answer. "Both work" is not a sentence this
site publishes. The reader came for the judgement the author already made.

### Say when not to use it

Every guide states its own boundary. Sometimes that is a section; more often it
is a sentence in the opening — compression is off by default *because something
in front of the application usually compresses already, and encoding twice
benefits nobody*. Either form is fine. Its absence is not.

`check_docs.mjs --only=shape` flags pages where it can find no such statement.
That check reads wording, so treat a hit as a question rather than a verdict:
open the page and see whether the boundary is there in words the pattern missed.

### Prerequisites go at the top

A page that assumes something states it in the first screen, as a sentence
naming what the reader needs and where to get it. The tutorial does this in a
`:::note[Before you start]` aside. Never introduce a difficulty badge —
see `references/terminology.md`.

### Terminology is fixed

`registered` and `discovered` routing. `.pw.html` and `.pw.sql` as typed
template and query languages. `_pw_gen.go` as build output nobody edits.
`pw.ServeMux` as a type alias for `net/http.ServeMux`, not a wrapper.

Each of those names a real distinction, and `references/terminology.md` explains
what the wrong word tells the reader that is false. `--only=terms` catches the
phrasings that have already shown up in drafts.

### Both locales, or the reader silently gets the wrong one

Every English page needs its Japanese counterpart at the mirrored path.
Starlight does **not** fail the build when one is missing — it falls back, so
`/ja/appendix/diagnostics/` currently serves English prose inside
`<html lang="ja">` with no notice to the reader at all. The parity check is the
only thing that catches this.

When you write Japanese, load the `japanese-cognitive-rhythm-writing` skill;
for English, `english-cognitive-rhythm-writing`. The Japanese page is a
rewrite at the same density, not a sentence-by-sentence translation.

### Before you call it done

```bash
node .claude/skills/docs-quality/check_docs.mjs
cd website && npm run build
```

The build is the only check on Markdown that Starlight itself rejects, and on
sidebar slugs — an explicit sidebar entry pointing at a missing page fails the
build with `AstroUserError: The slug ... does not exist`.

## Mode 2 — auditing existing pages

Run the checker, read the pages, then report findings **ordered by what a reader
loses**, not by how easy they are to fix.

1. **Wrong.** The page contradicts the code, or the code example does not
   compile. A reader who trusts it loses an afternoon.
2. **Broken.** Dead link, dead anchor, retired URL, missing Japanese page.
   Mechanical, and the checker has already found all of these.
3. **Unusable.** The example is a fragment, the reader cannot tell which option
   to pick, the page never says when to stay away. The information is present
   and the reader still cannot act on it.
4. **Degraded.** A table where an argument belonged, an enumeration that should
   live in `reference/`, a missing prerequisite line, drifted terminology.
5. **Rough.** Density, cadence, an unlanded abstraction.

For each finding give the file and line, what the reader loses, and the specific
fix. "Improve the prose" is not a finding. "The three storage options are listed
with no recommendation, so a reader who does not already know picks the first
one" is.

Two honest verdicts are worth stating out loud when they apply, because an
auditor under pressure to produce findings will otherwise invent work:

- **This page is fine as it is.** Configuration surfaces and reference tables
  usually are.
- **This page is the wrong shape entirely** and needs rewriting rather than a
  list of patches.

`references/exemplar-tutorial-getting-started.md` is the density target for
tutorial chapters; `references/exemplar-guide-compression.md` is the target for
a configuration-surface guide. Compare against whichever one applies rather than
against an average of the site.

## What the checker looks at

`links` resolves every root-absolute Markdown link against the actual route map,
including `#anchors`, and reports a retired route with the page that replaced it
(the reorganisations that moved `start/getting-started/` and
`guides/configuration/` are recorded in the script, and further deletions are
read out of git history). It also catches a Japanese page linking into the
English tree, which drops the reader out of their locale.

`frontmatter` requires `title` and `description`, checks the description is a
sentence rather than a fragment or a paragraph, and reports a `sidebar.order`
that disagrees between the two locales — which would list the same group in two
different orders.

`parity` reports a missing counterpart in either direction, and a heading count
that has drifted between the locales.

`tailwind` scans code blocks on pages where the reader has no Tailwind build for
utility classes that would not resolve. Chapter 1 of the tutorial declines
Tailwind at `pw init`, so a `class="text-3xl font-bold"` in that chapter is
markup the reader cannot reproduce. The page list is `TAILWIND_OFF` at the top
of the script.

`tutorial` walks the four chapters in order, tracks the symbols each code block
declares per file path, and flags a symbol that disappears from a later full
listing without the chapter mentioning it. A removal the chapter names — in a
code comment or in the paragraph under the block — is not reported.

`sidebar` compares `astro.config.mjs` against the filesystem in both directions:
a page in no group, an autogenerated directory that is empty or gone, an explicit
slug with no file, a directory with no `ja/` sibling.

`terms` and `shape` are described above.

## Gotchas

**Anchors are github-slugger's, and it does not collapse spaces.** A heading
written `cookie — no storage at all` anchors as `cookie--no-storage-at-all`,
with two dashes, because the em dash is deleted and each surviving space becomes
its own dash. Inline code in a heading contributes its text: `### \`[observability.otel]\``
anchors as `#observabilityotel`. Both of these are already used in the docs and
both look like typos. They are not.

**A missing Japanese page produces a page, not an error.** Covered above; worth
repeating because a green build is the reason this goes unnoticed.

**`--path` filters the pages being reported, not the route map.** Links are
still resolved against the whole site, which is what you want — a scoped run
will still tell you that the page you are editing points at something that does
not exist.

**The `shape` and `tutorial` checks are wording-sensitive by design.** They are
`warn` and `info` and they never fail the build. Treat them as a reading list.

**`npm ci` in `website/` warns about unapproved install scripts** for `esbuild`
and `fsevents`. The build works anyway; nothing needs approving.

## Extending the checks

A new rule needs three edits, and the third is what keeps it alive: the pattern
in `check_docs.mjs`, a planted defect in `tests/fixture/`, and a line in the
`EXPECTED` table of `tests/run_tests.mjs`. The fixture is a miniature site with
one deliberate fault per check, so `run_tests.mjs` proves both that each check
fires and that the clean pages beside them stay quiet. It also sweeps the real
site for link errors, which is the regression guard on the slugger.

## Evaluating changes to this skill

`tests/evals/` holds one case per mode — a deliberately weak page for the audit
mode, a feature specification for the writing mode — each with the findings or
the page properties a good answer has to produce. `tests/evals/README.md` says
how to run them.
