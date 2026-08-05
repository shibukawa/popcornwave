# Expected result — writing mode, `write-01-input.md`

## How to run it

Give the agent the specification and a request phrased the way the trigger has to
survive in real use — no mention of documentation style, and no mention of this
skill:

> I've just landed the rate limiting middleware. Here's the spec. Update the
> docs.

Two things are under test. Whether the skill loads at all from a request that
only says "update the docs", and whether what comes out follows the house style.

## Where the page belongs

`website/src/content/docs/guides/backend/rate-limiting.md` and its counterpart at
`website/src/content/docs/ja/guides/backend/rate-limiting.md`. It is a
cross-cutting middleware serving the application rather than the page, which puts
it beside Sessions and Cookies. Placing it under `guides/deployment/` is
defensible if the agent argues for it. Placing it under `reference/` is not — a
reference page could not carry the fail-open decision.

Both files, or the Japanese reader gets English prose under `lang="ja"` with no
notice. An agent that writes only the English page has failed this case.

## The page must

**Open on the problem, not the feature.** The spec hands over an opening: an
application in front of a database has a request rate above which it stops
serving anyone, and one client can reach it alone. A page that opens "Popcorn
Wave provides rate limiting" has already lost the first screen.

**Give an example that runs when pasted.** The TOML block, complete, with the
`enabled = true` the reader actually needs — the shipped default is `false`.

**Recommend rather than survey.** The spec contains three decisions and each has
a defensible default. Memory or redis: memory for one instance, redis as soon as
there are two, and the page should say the multiplication rule out loud rather
than describing the stores side by side. IP or session keying: IP unless there is
already a session middleware. `trust_proxy`: false unless a proxy you control
sets the header, because a forged `X-Forwarded-For` buys an unlimited allowance.
A page that lists these without landing on one has failed the central rule.

**Say when not to use it.** The spec supports at least two boundaries: a CDN or
ingress already limiting makes this redundant, and a route needing its own limit
gets nothing from a middleware with no per-route form.

**Carry the fail-open decision as reasoning.** Redis unreachable means requests
pass unlimited. That is a designed tradeoff with a stated reason, and it is
exactly the kind of thing that dies in a table cell. It belongs in sentences.

**Explain `skip_paths` through its consequence.** The default is not arbitrary —
a limiter that rejects `/readyz` removes the instance from the load balancer
under the load it existed to survive. Reproducing the default without the reason
is a partial pass.

**Link to `/guides/deployment/operational-endpoints/` and
`/guides/architecture/configuration/`**, both of which exist, and both of which
`check_docs.mjs --only=links` will verify.

## The page must not

**Table every configuration key.** Eight keys with types and defaults is
`reference/configuration.md` work. Two or three that carry a decision belong
here; a link covers the rest. An agent that reproduces the whole TOML block as an
example and then explains the two or three interesting keys in prose has done
this correctly — the block is an example, not an enumeration.

**Introduce a difficulty badge.**

**Say "wrapper", "classic", "modern", or "legacy".** Nothing in the spec invites
it, which is the point: the terminology has to hold when the agent is not being
tested on it.

**Claim a per-route form, an algorithm choice, or an override.** The spec rules
out all three, and inventing them is the most damaging failure available here,
because it is the one a reader would act on.

## Then check it mechanically

```bash
node .claude/skills/docs-quality/check_docs.mjs --path=rate-limiting
cd website && npm run build
```

Clean, in both locales. A `shape` info finding about a missing boundary
statement is a real miss on this case, since the spec supplied the material.
