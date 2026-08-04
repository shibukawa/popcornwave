# Expected result — audit mode, `audit-01-input.md`

The page is a fictional rate-limiting guide holding one planted defect per rule
the skill enforces. Nothing in the repository implements rate limiting, so the
fixture cannot drift into a real page.

## How to run it

Give the agent the input file and ask it to audit the page against the docs
house style. Do not name the skill; the point is partly to see whether the
description triggers.

```bash
cat .claude/skills/docs-quality/tests/evals/audit-01-input.md
```

An agent that copies the file to `website/src/content/docs/guides/frontend/`
first can also run the checker over it, which should surface the mechanical
subset below. Delete the copy afterwards.

## Must find (a miss here is a failure)

**Terminology.** "classic router and the modern router" — the two routers are
registered and discovered, and calling one classic tells a reader it is on its
way out. "the `pw.ServeMux` wrapper" — it is a type alias for
`net/http.ServeMux`, and "wrapper" sends the reader looking for framework
routing behaviour that does not exist. "edit it if you need to change the
behaviour", about `ratelimit_pw_gen.go` — generated files are build output and
`pw generate` overwrites them, so following this instruction loses the reader's
work.

**Broken link.** `/guides/configuration/` was retired when the guides were
grouped by the reader's job. It is `/guides/architecture/configuration/`.

**Difficulty badge.** `badge: advanced` in the frontmatter. The site has no
difficulty labels.

**Frontmatter description.** "Rate limiting." repeats the title and tells a
search result nothing.

**No recommendation.** Three algorithms and three keying strategies are listed,
followed by "All of them work. Choose the one that fits your use case." — which
hands the decision back to the reader who came for it. A correct finding names
what is missing: which one to take by default, and the condition that changes
the answer.

**No boundary.** Nothing says when not to rate-limit, or when a proxy in front
of the application already does it.

**Exhaustive option table in a guide.** Thirteen keys with types and defaults is
`reference/` work. The guide should keep the two or three that carry a decision.

**Incomplete example.** `mux.Use(ratelimit.New(cfg))` is one line with no
imports, no `cfg`, and no configuration file — it cannot be pasted and run.

**Tailwind in a code block.** `rounded-lg bg-red-50 p-4` in a page that never
establishes a Tailwind build.

## Should find

**Opening is circular.** "Rate limiting is a middleware that limits requests.
This page explains rate limiting." says nothing and spends the reader's first
screen.

**"The memory store is faster. The redis store is shared."** states two facts and
withholds the consequence — that a single instance can use memory and a fleet
cannot, which is the actual decision rule.

**Missing prerequisite line.** The Redis keys assume a running Redis; the page
never says so.

**No Japanese counterpart.**

## Must NOT flag

**The store comparison table.** Two stores against two properties is genuinely
coordinate. The fix is to add the consequence *around* it, not to dissolve the
table into prose.

**The 429 default or any other factual default.** Nothing in the fixture is
wrong about the (fictional) behaviour; an auditor inventing correctness findings
here is over-reaching.

## Scoring

Every "must find" present, with a file reference and a stated consequence for
the reader, and neither "must not flag" item raised. Findings ordered by what
the reader loses — the three terminology errors and the dead link outrank the
prose problems — rather than grouped by how easy each is to fix.
