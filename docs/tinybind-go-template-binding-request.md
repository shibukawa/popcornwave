# Change request: binding an external call's result in a template

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** `github.com/shibukawa/tinybind-go` v0.5.9
**Date:** 2026-08-14
**Status:** open

## Scope

This one is entirely yours. There is nothing to build downstream and nothing
here is blocked on us — it is a template language change, and we have no way to
approximate it. One ask, and it is deliberately the smallest version of itself.

## Summary

A synchronous `external` can be called from markup but its result cannot be
named. Every place the result appears is another call. That is correct today and
costs little, because the results being called for are small.

It stops being cheap the moment a component fetches. We would like to move
toward a component that takes a primary key, loads its own data, and caches the
fetch and the render together as one unit — and under the current grammar such a
component calls its loader once per mention of the loaded value. A component
rendering four fields of one record makes four calls.

So the ask is a binding form:

```
{let record = LoadData(id)}
  <h1>{record.title}</h1>
  <p>{record.summary}</p>
{/let}
```

One name, one call, one value. The spelling is yours to choose.

## What the language does and does not have today

It already binds names in two places, so this is a new use of an existing idea
rather than a new kind of thing:

| Construct | Binds | Scope |
| --- | --- | --- |
| `{for post in posts}` | the loop variable | the loop body |
| `{await user = LoadUser(id), posts = LoadPosts(id)}` | one or more `external async` results | the primary subtree |
| a synchronous `external` | **nothing** | — |

The `await` clause is the shape we want, and it is unavailable for this: it binds
`external async` calls specifically, it opens a boundary, and `fallback` is
required. Declaring a fast local lookup `async` to get a variable out of it means
paying for a streamed region, a placeholder, and a commit point to avoid typing a
call twice. That is not a workaround we would recommend to anybody.

Each mention compiles to its own op holding its own closure, so nothing collapses
two identical calls today, and we are not asking for anything that would.

## Why the repeat is safe, and why that is the argument

`rule:render-external-query-semantics` already requires an external to be a
repeatable data query: deterministic for declared inputs, no externally visible
mutation, no exactly-once side effect. It says outright that a full render, a
partial navigation, a cache miss, a retry, and a direct boundary update may each
execute the same component again.

That rule is what makes this ask small. **Calling once instead of four times
changes no behaviour the language guarantees**, because the language already
declines to guarantee a call count. This is an efficiency and legibility change
against a contract that already permits it, not a semantic change that needs new
rules around it.

It is also why we are *not* asking you to collapse identical calls
automatically. Doing that would make the call count depend on an optimiser, and
authors would start relying on a number the rule deliberately leaves open. A
binding puts the count in the author's hands and in the source, where it can be
read.

## Ask — a binding form for a synchronous external

The one design question we cannot answer for you is whether it is a block or a
statement.

**A block, scoped like every other binding here.** `{let a = f()}` … `{/let}`,
with the name visible in the subtree. Consistent with `for` and `await`, and the
scope is a subtree exactly as those are, so the analyses you already run — cache
eligibility, boundary placement, slot rules — see a shape they already
understand.

**A statement, scoped to the end of the enclosing block.** `{var a = f()}` with
no closer, the way the request was first written. Much less noise at the call
site, especially for two or three bindings in one element, and it reads like Go.

We would take the block form, and the reason is not consistency for its own
sake. Every construct in this language today is a node whose effect is contained
in its own subtree. A statement whose effect extends past its own node to its
later siblings would be the first thing that is not, and that changes what a
traversal has to carry rather than adding another case to it. If that turns out
to be cheap in your tree, the statement form is the nicer one to write and we
would be glad to be wrong.

Whichever it is, we would want the name to be an ordinary value afterwards:
readable in interpolation, in attributes, in an `if` condition, as a component
argument, and as an argument to another external in the same scope.

Several bindings in one construct would be welcome — `{let a = f(), b = g()}`,
which is what `await` already accepts — but that is still one value per name.

**No multi-value returns.** This request was first drafted asking for
`{var a, b = LoadData(id)}`, one call yielding two values, and we have withdrawn
it. It lands on the declaration grammar rather than the markup grammar, needs a
tuple-ish type the language has no other use for, and raises a question about
what happens when only one of the two is read — all to avoid a Go function
returning a record containing both. The record is the better answer anyway,
because it has a name: a second caller can accept it as a parameter, which a
tuple cannot offer.

For the same reason, please do not read `a, b = f()` as destructuring a record
either. It looks identical to a multi-value return, means something else, and an
author who guesses wrong gets a type error at a position that cannot tell them
which of the two they thought they were writing.

## What this unblocks downstream, for context

We shipped a data result cache in Popcorn Wave: `pw.Memo` over a store named in
configuration, keyed on a generated method, with concurrent misses coalesced. It
works, and it sits in the handler.

What the owner actually wants is one layer up — a component declaring the
primary key as its parameter, loading inside itself, and carrying one `@cache`
that covers the load and the render together. Today the render cache saves the
rendering and the data cache saves the fetching, and a page that is slow for both
reasons is configured in two places for what an author thinks of as one thing.

We are not asking for that component yet; the caching side of it needs its own
design and it is not clear to us how the key would reach both halves. But the
binding is a prerequisite for it either way, and it is worth having on its own —
today it is the reason a component cannot honestly fetch anything.
