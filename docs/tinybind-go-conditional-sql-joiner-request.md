# Change request: infer the joiner from the operator already written beside a conditional block

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.5.14
**Date:** 2026-08-17
**Status:** accepted and implemented in v0.5.15, 2026-08-17. The release broadened joiner recognition (every `AND`/`OR` at the innermost open group's item depth, not only one adjacent to an elidable node), landed the comma clauses in the same change, excluded `CASE` rather than diagnosing every case in it, and corrected the join-`ON` premise in Open Questions below — `ON` was already classified boolean. See the upstream catalog for the shipped spec.

## Summary

`requirement:sql-template-v1` already describes the outcome, under `structured_lists`:

> `where: AND children by default; explicit and/or groups; omit when empty for SELECT`
> `set: manage commas; require a statically provable non-empty item set per rule:sql-static-mutation-safety`
> `order_by: static branches or enums; manage commas and empty clause`

What is implemented is flat text. `emitNodes` (`templates/sqlbind/generate.go:511`) emits a text node verbatim inside a plain Go `if`, with no separator logic anywhere, so the operator joining two conditions is SQL bytes the author owns and the author cannot get right. We would like the gap closed **without adding any syntax** — the operator the author already writes beside the block becomes the joiner.

Three parts, which only work together:

1. **An operator token adjacent to a node that is not provably non-empty becomes a joiner rather than text.** `alwaysEmits` (`templates/sqlbind/mutation.go:148`) already decides "provably non-empty" and is already recursive through `{if}`/`{else}` and through a `sql.predicate` body. Adjacency is structural — the token immediately before or after such a node, at the group's own depth — so nothing is inferred from free text in general.
2. **Groups read from the SQL the author already wrote:** a clause keyword, and a grouping parenthesis inside a region one opened.
3. **Elision by writing late, never by erasing.** A group's opening keyword or parenthesis, and a pending joiner, are written the moment a fragment inside actually emits. A group that stays empty was never opened, so nothing has to be taken back.

**No grammar change, no new directive, no migration.** The whole request is three tokens the builder decides to withhold: a joiner, a parenthesis pair, and a clause keyword.

## The template that should already work

```sql
export statement SearchUsers(
  name: string, city: string, minAge: int,
  hasName: bool, hasCity: bool, hasAge: bool, staffOnly: bool
): sql.many<User> {
SELECT id, name, city, age
FROM users
WHERE
  {if hasName}name LIKE {name}{/if}
  AND {if hasCity}city = {city}{/if}
  AND ({if hasAge}age >= {minAge}{/if} OR {if staffOnly}role = 'staff'{/if})
ORDER BY id
}
```

Read that with every condition true and the `{if}` wrappers deleted, and it is the statement it renders: `WHERE a AND b AND (c OR d)`. **That is the property we are really asking for** — a conditional predicate whose source reads as the fully-populated SQL, with the conditions punched out of it rather than woven into it. Every operator sits between its two operands, in the enclosing text, belonging to the pair rather than to either side.

It is broken in thirteen of its sixteen branch combinations today. Three of them:

| conditions | today | asked for |
| --- | --- | --- |
| all true | `WHERE name LIKE $1 AND city = $2 AND (age >= $3 OR role = 'staff')` | unchanged |
| `hasCity` only | `WHERE AND city = $1 AND ( OR )` | `WHERE city = $1` |
| none | `WHERE AND ( OR )` | no WHERE at all |

Note what the first row means: **when the operator is not dangling, the rendered SQL is byte-identical to today.** That is the whole compatibility story, and it is why we would rather infer than mark.

## Why we are not asking for `{and}` / `{or}` markers

We drafted this request with explicit markers first, on the reasoning that inferring a boolean operator out of SQL text is the kind of heuristic `decision:tinybind-sql-runtime` records as *removed* rather than refined (`_tinybindSafeMutation runtime token heuristic`). On reflection that reasoning does not transfer, for two reasons.

**The blast radius is a boundary, not a body.** The removed heuristic scanned whole statements for tokens that might indicate a safe mutation. This looks at one token, on one side of one node, at one depth, and only when that node is not provably non-empty. Everything else in the statement is untouched text. `alwaysEmits` is not a guess; it is the same proof `checkMutationSafety` already trusts to decide whether a DELETE may run.

**A marker would change working templates and inference cannot.** An operator that is not dangling renders identically, and an operator that is dangling renders SQL the engine rejects. So inference either fixes a source or leaves it alone — there is no third outcome, and no branch combination in which a template that works today stops working. A marker, by contrast, requires editing every existing conditional predicate, and until it is edited the old spelling stays quietly wrong.

So this request carries no migration section, because there is nothing to migrate — and no grammar addition, no new node kind, no scope name, and no rewriting tool to build and then retire.

## Ask 1 — what counts as a joiner

An `AND` or `OR` (or, in a comma-separated clause, a `,`) becomes a joiner when **all** of these hold:

- it sits immediately before or after a node for which `alwaysEmits` is false — an `{if}` without a both-branches-emitting `{else}`, or a `sql.predicate` call whose body is not provably non-empty;
- it is at the depth of the group it would join, per the `sqlLexer` depth that `sqlscan.go:11-90` already carries across text nodes;
- the group it would join is classified for that separator: `AND`/`OR` in a clause `fmtclause.go:55` calls `boolean`, `,` in one it calls `comma`.

"Immediately" covers four positions, and they are one case rather than four — the trailing token of the text before the node, the leading token of the text after it, and the leading and trailing tokens of a branch body:

```sql
{if a}x = {a}{/if} AND {if b}y = {b}{/if}   -- canonical: between the two, enclosing text
{if a}x = {a}{/if} {if b}AND y = {b}{/if}   -- accepted: leading the branch
{if a}x = {a} AND{/if} {if b}y = {b}{/if}   -- accepted: trailing the branch
```

**The first is the form we would like the documentation to teach**, for the reason above: an operator in the enclosing text sits where it sits in the finished statement, so the source reads as the SQL. An operator inside a branch reads as part of that condition, which is the wrong shape for something that joins two.

The other two stay accepted, and we would rather they were not made errors. They are what `docs/sqlbind.md:367` teaches today and what `generate_test.go:19` and `mutation_test.go:60` contain, so rejecting them would leave the exact template shape the documentation recommends still broken, and reintroduce the migration this design otherwise avoids. Accepting all three costs one comparison per boundary — the analysis looks at the token on each side of the node either way.

We looked at whether the formatter could normalize the in-branch forms into the canonical one and concluded it cannot: moving a token across an `{if}` boundary changes the AST, which `rule:template-format-fidelity` forbids under `parse_stability` and `no_semantic_edits`. That invariant is load-bearing for idempotence, and we are not asking you to relax it. The canonical form is a recommendation the documentation carries, not something a tool enforces.

An operator anywhere else stays text. In particular `WHERE a = {a} AND b = {b}` with no conditions has no boundary, so nothing about it changes.

### Three exactness rules

These are the cases where an adjacent operator is *not* a joiner, and we would rather have them written down than discovered:

**`BETWEEN` closes with an `AND` that is never a joiner.** `BETWEEN` is a fixed two-operand form, so one bit of scanner state — opened by `BETWEEN`, closed by the next `AND` at that depth — settles it exactly. This is grammar, not a heuristic. It matters for `WHERE x BETWEEN {lo} {if hi}AND {hi}{/if}`, which is already broken on the false branch and should be reported rather than silently made worse.

**A parenthesis preceded by a word is a call or list paren, not a group.** `rule:sql-template-layout` already draws this line for layout — *a parenthesized value list, function argument list, or IN list is data, not a statement* — and the same test applies here. `sqlLexer` tracks depth but does not yet distinguish the two, and this ask needs it to.

**A conditional boundary inside a `CASE` expression should be refused.** `CASE WHEN` opens a boolean region that is neither a clause keyword nor a parenthesis, so the group stack has nothing to attach to and a vanishing `WHEN` condition would leave `CASE WHEN THEN`. We would rather have an error naming `CASE` than partial support. If you would rather model it, we have no objection; we just do not want it to fall through.

## Ask 2 — groups read from the SQL

A group opens at a clause keyword or at a grouping parenthesis inside a region one established, and closes at the matching parenthesis or the clause terminator. `sqlscan.go` carries depth across text nodes and skips literals, quoted identifiers, comments, and dollar-quoted regions; `mutation.go:26-33` names the terminator sets; `fmtclause.go:41-70` classifies the clauses. Every substrate exists.

We are **not** asking you to emit a parenthesis the author did not write. A generated grouping paren would make the SQL that runs differ from the SQL in the template, which is the line `docs/sqlbind.md:71` draws about dialect rewriting, and we think it holds here unchanged. The builder should only withhold a parenthesis the author *did* write.

One case must stay unresolvable rather than guessed: a parenthesis pair opening inside one branch and closing outside it. `walkClause` (`mutation.go:101-104`) already treats branches that close different numbers of groups as not valid SQL on both paths, and we would like an error rather than a paired guess.

## Ask 3 — elision by writing late

A stack of frames in `Builder`, one per open group. Each frame holds its opener text, whether that opener has been written, and a pending joiner.

- **`OpenGroup(opener)`** — push. Write nothing yet.
- **`Joiner(sep)`** — record it as the frame's pending joiner. A no-op when the frame has written no item yet, which is what makes a leading operator vanish.
- **`Item()`** — emitted immediately before every fragment that can write text or bind a value. Walk from the outermost unwritten ancestor inward, writing each ancestor's opener after flushing *that ancestor's parent's* pending joiner; then flush this frame's pending joiner; then mark the frame written.
- **`CloseGroup()`** — write the closing parenthesis only if the opener was written. Drop a pending joiner nothing followed, which is what makes a trailing operator vanish.

A nested group registers in its parent only by having filled, so an empty one takes its parentheses *and* the joiner that attached it. That is row 2 of the table above, and it falls out rather than needing a rule of its own.

The generator's only new decision is **which call a sliced token becomes**: an operator on a boundary is emitted as `Joiner`, the same token anywhere else as `WriteString`. Both slices come from offsets the scanner already reports.

**Placeholder numbering and `Args` are unchanged in every branch combination.** The only text elided is an opener, a closer, and a joiner, none of which binds; a group that vanishes registered no item, and `Item()` precedes anything that can bind, so a vanished group bound nothing. The invariant `Arg` (`sqlbind/statement.go:115`) keeps — appending the argument and writing its placeholder are one operation — is untouched.

Two details we would get wrong if we left them implicit:

**The text split.** An opener is the whitespace run preceding the keyword or parenthesis, through that token; the item text begins immediately after. Keeping the leading whitespace with the opener leaves no double space when the group survives and no missing space when it vanishes — the reverse split fails both ways. `generate_test.go:34` shows the stakes: the space before `{if active}` is currently the only thing separating `)` from `AND`.

**No runtime reparse.** We are explicitly not asking for a post-render cleanup pass. A runtime scan cannot tell a dangling operator from one inside a literal without redoing `sqlLexer` on every call, and `requirement:sql-template-v1` already forbids the shape: *a generated runtime check for a condition decidable at generation time*.

### Mutation safety must not relax

`rule:sql-static-mutation-safety` stays exactly as it is, and we would rather this request be rejected than land with it weakened. An UPDATE or DELETE whose predicate can empty out on any branch stays a generation error; dropping an empty WHERE there turns one false branch into a full-table delete, which is what the proof exists to prevent.

The only change we ask for is that `proveClause` run against the **group** rather than against the keyword, and that a token recognized as a joiner count as filling nothing. A vanishing clause is a SELECT, HAVING, subquery, and join-`ON` affordance only — which is what `requirement:sql-template-v1` already says with `omit when empty for SELECT`.

Note this cuts the other way too, in your favour: `mutation_test.go:56` (`DELETE FROM users WHERE {if flag}id = {id}{/if}`) must keep failing, and it does, because the proof is about the group being provably non-empty rather than about whether the text renders.

## Compatibility

- **A template whose operators never dangle renders byte-identical SQL.** Whitespace normalisation around an elided joiner is the only possible difference, and only in the branches that are broken today.
- **A template whose operators do dangle changes from invalid SQL to valid SQL.** We are treating that as a fix rather than a behaviour change, and we think a release note is the right weight for it.
- **No new node kind, no new directive, no scope name.** `analyzeNodes` (`compiler.go:378`) rejects unknown node kinds and needs no new arm. On our side that means `rule:template-grammar-scopes` and the tokenizer snapshot it holds over every `.pw.sql` in our repository are untouched, which is the largest cost any syntax addition carries for us.
- **`OpenGroup`, `Joiner`, `Item`, `CloseGroup`** are new methods on a `Builder` only generated code drives. Generated bodies gain calls, no signature changes.
- The walkers that need to admit the new distinction are ones that already walk: `walkClause` and `alwaysEmits`, `isReadOnly` / `topLevelVerb`, `validateStaticResultShape` and `staticSQL`, and the formatter, which already treats `,` (`print.go:292`) and `AND`/`OR` (`print.go:300`) as the item separators of a classified clause.

## Open questions

**Does a join `ON` get a group?** We think it should — a conditional join predicate has the same failures — but `fmtclause.go` does not classify `ON` as boolean today, so it is the one opener not already computed. Your call whether it lands in the same change.

**Which comma-separated clauses are in scope.** `,` elision is well defined wherever `fmtclause.go` says `comma`, but `SELECT` and `RETURNING` are in that set and conditional items there are already refused by `validateStaticResultShape` (`compiler.go:693`). We would expect no interaction, and would rather hear that confirmed than assume it. A `SET` list additionally meets the mutation proof, so it may want to land after WHERE rather than beside it.

**Should a dangling operator that inference cannot resolve be an error or left alone?** The `BETWEEN` and `CASE` cases above are the ones we know of. We prefer an error naming the construct, on the grounds that the template is already broken on some branch, but leaving it as today's verbatim text is also defensible and we would take it.

## What we are not asking for

- **Any new syntax.** No marker, no block form, no clause construct.
- **Generated punctuation.** The builder withholds; it never invents.
- **Post-render processing.** Nothing scans, trims, or rewrites the assembled string, at generation time or run time.
- **A relaxation of mutation safety.** Covered above; we would rather lose the request.
- **Conditional result columns,** or a general `{for}` in a clause. Both still forbidden, both for their existing reasons.
- **Dialect variation.** A boolean joiner and a parenthesis are not what a dialect differs about, so the frame protocol should be identical everywhere.

## What we can contribute

- **A consumer and a test surface.** We have the generation path (`internal/pwcli/generate.go`), a dev query runner that assembles statements through the same builder, and documentation pages that currently teach the hand-written operator as something the author must get right. We can implement against a prerelease and report where the inference is under-determined, as we did for the v0.4.7 update surface.
- **The branch-combination matrix.** The verification we want is a table test over every combination of a two- and a three-condition predicate, asserting SQL text and `Args` together, plus the nested-group, all-absent, `BETWEEN`, and call-paren cases. We can write and contribute it against a prerelease.
- **Byte-identity evidence.** Because a non-dangling operator should render unchanged, every existing conditional-SQL test is a regression test for this change. We can run our generated output before and after and report any diff that is not in a branch that was broken.

## Related concepts

**Yours:** `requirement:sql-template-v1`, `rule:sql-static-mutation-safety`, `rule:sql-top-level-keyword-scan`, `rule:sql-placeholder-emission`, `rule:sql-template-layout`, `rule:sql-cardinality-body-agreement`, `concept:typed-template-language`

**Ours:** `requirement:sql-conditional-predicate-composition`, `decision:sql-boundary-joiner-inference`, `rule:sql-predicate-group-elision`, `flow:sql-generation`, `decision:tinybind-sql-runtime`
