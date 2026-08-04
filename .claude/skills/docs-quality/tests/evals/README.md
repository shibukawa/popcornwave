# Evals

Two cases, one per mode of the skill. Neither is automated — both are judged by
reading the agent's output against the expectations file, because both ask about
prose.

The mechanical half is automated and lives one directory up:

```bash
node .claude/skills/docs-quality/tests/run_tests.mjs
```

## The cases

| Case | Input | Judged against |
| --- | --- | --- |
| Audit | `audit-01-input.md` — a fictional rate-limiting guide with one planted defect per rule | `audit-01-expected.md` |
| Writing | `write-01-input.md` — a specification for a feature that has just landed | `write-01-expected.md` |

Both concern rate limiting, which the framework does not implement. That is
deliberate: neither fixture can drift into something a reader might mistake for
real documentation, and neither can be answered by copying an existing page.

## Running one

Start a fresh session so the skill has to trigger on its own, and give the agent
the request as a user would phrase it. The audit case:

> Review this page against our docs standards.

The writing case:

> I've just landed the rate limiting middleware. Here's the spec. Update the
> docs.

Neither prompt names the skill. Whether it loads is half of what is being
measured — the writing case in particular is the undertrigger risk, since
"update the docs" arriving at the end of an implementation session is the moment
the skill is most likely to be skipped.

## Judging

Each expectations file separates findings the agent **must** produce from ones it
**should**, and names what it **must not** raise. The must-not list is the one
that catches an agent generating findings to look thorough — an auditor that
flags the legitimate comparison table in the audit fixture has misread the rule
about tables, and that misreading would cost real pages their clearest sections.

Record failures as changes to `SKILL.md` or to `references/`, then re-run. A rule
that an agent repeatedly misses is usually a rule stated without its reason.
