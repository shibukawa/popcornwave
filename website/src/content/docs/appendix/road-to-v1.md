---
title: Road to 1.0
description: The API changes already designed and waiting on a Go language feature, with each call site shown before and after.
sidebar:
  order: 4
---

A version number below 1.0 means what it usually means: the surface can still
move, and a release is allowed to take a name back rather than carry it
forever. One batch of changes is already designed and waiting, though, and it
is not waiting on this project. This page records it — each call site as it
reads today, and as it will read afterwards — so that none of it arrives as a
surprise.

## What is waiting, and on what

A Go method may not declare its own type parameters. Wherever the framework
needs one — a typed read, a typed write, a typed configuration accessor — the
operation has to be a package function, whatever the design would prefer:

```go
// The store is the receiver in everything except the syntax.
quote, err := pw.Memo(ctx, store, QuoteKey{Pair: pair}, fetchQuote)
```

When the language allows a method to take a type parameter, each of these
becomes the method it was always describing.

The trigger is two releases rather than one: a Go release carrying the feature,
**and** a TinyGo release carrying it. A conversion available only on host Go
would split the build rather than tidy it. The expectation is Go 1.27 with
TinyGo 0.42 — roughly February 2027, given Go 1.26 in August 2026 — but methods
with type parameters are not a committed language feature. If it slips, this
page slips with it, and nothing described here breaks in the meantime.

**Every move below is additive.** The method becomes the body and the existing
function stays as a deprecated wrapper, so a project migrates call site by call
site, or not at all, and no compiler error forces the edit. Each entry is a call
shape: nothing stored, generated, or on the wire changes.

## The data cache

[`pw.MemoStore`](/guides/backend/data-cache/) already resolves a store to a
handle, and it does so ahead of the language on purpose. With the store held in
a value, the operations move onto it without touching the line that acquired it.

```go
// Today
store, err := pw.MemoStore(r, "rates")

quote, err := pw.Memo(ctx, store, QuoteKey{Pair: pair}, fetchQuote)
if pw.MemoHas(ctx, store, key) { /* … */ }
pw.MemoSet(ctx, store, key, quote)
pw.MemoInvalidate(ctx, store, key)
```

```go
// Afterwards — the first line is unchanged, which is the point
store, err := pw.MemoStore(r, "rates")

quote, err := store.Get(ctx, QuoteKey{Pair: pair}, fetchQuote)
if store.Has(ctx, key) { /* … */ }
store.Set(ctx, key, quote)
store.Invalidate(ctx, key)
```

`MemoInvalidateScope` and `MemoInvalidateTag` take no type parameter and could
have been methods all along; they move with the rest so that the store reads one
way rather than two.

## A Firestore transaction

This is the one whose value is more than tidiness. Writes are already methods on
the transaction while typed reads are not, so a single transaction is written
two ways in adjacent lines:

```go
// Today
tx.Store(user)
user, err := firestorebind.LoadTx[User](ctx, tx, key)
```

```go
// Afterwards
tx.Store(user)
user, err := tx.Load[User](ctx, key)
```

`LoadTx`, `LoadAllTx`, and `QueryPageTx` are the three, and what changes is more
than the spelling: the transaction boundary stops being an argument and becomes
the receiver, so the call states what it is inside.

Reaching a transactional read through the transaction value rather than through
a context is a separate decision, and it survives the change untouched — a
context-carried handle would make one call site mean two different things
depending on which context reached it.

## The DynamoDB and Firestore handles

Popcorn Web wraps neither store, so the `On` entries are what an application
author literally writes, and the handle is a concrete type waiting to be a
receiver.

```go
// Today
h, err := dynamo.Handle(ctx)
note, err := dynamobind.LoadOn[Note](ctx, h, "note", key)
err = dynamobind.StoreOn(ctx, h, "note", note)
```

```go
// Afterwards
h, err := dynamo.Handle(ctx)
note, err := h.Load[Note](ctx, "note", key)
err = h.Store(ctx, "note", note)
```

Read the last line twice. An operation whose type is inferable from its argument
loses its type argument entirely, so storing, storing many, and removing end up
as plain calls.

The full set is `LoadOn`, `LoadAllOn`, `StoreOn`, `StoreAllOn`,
`StoreReturningOn`, `RemoveOn`, `RemoveReturningOn`, `UpdateOn`, `QueryPageOn`,
`QueryOn`, `ScanPageOn` and `ScanOn` for
[DynamoDB](/guides/storage/dynamodb/), and `LoadOn`, `LoadAllOn`, `StoreOn`,
`StoreAllOn`, `InsertOn`, `InsertAllOn`, `UpdateOn`, `RemoveOn`, `RemoveAllOn`,
`QueryPageOn` and `QueryOn` for [Firestore](/guides/storage/firestore/). The
context-resolving forms beside them — `dynamobind.Load[Note](ctx, …)` — stay
exactly as they are, having no receiver by design.

## An isolated test configuration

```go
// Today
testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
	middleware.CSRF.Enabled = false
})
app := testutil.Get[AppConfig](config)
testutil.Set(config, app)
```

```go
// Afterwards
config.Update(func(middleware *pw.MiddlewareConfig) {
	middleware.CSRF.Enabled = false
})
app := config.Get[AppConfig]()
config.Set(app)
```

Two of those three already infer their type from an argument, and they are still
blocked: a method may not declare a type parameter even when nothing has to be
written at the call site. So all three of
[`testutil`](/productivity/testing/)'s configuration operations wait together.

## What does not move

The context-resolving accessors stay functions permanently, because they have no
receiver by design — that is the half of each pair that reads a value out of a
`context.Context` rather than out of a handle. Constructors stay functions for
the same reason.

One entry is blocked by more than the language: `sqlbind.ScanRows` takes a row
cursor, which is an interface, and no package may give a method to a type it
does not define. It stays a function however Go changes, and that is worth
recording rather than rediscovering.

The generated layers move too — the HTML builder's loop, await, live and
provider entries, the JSON parser's `ParseSlice` and `ParseMap`, and
`sqlbind.AppendValues`. Nothing there is code an application reads or edits, so
those changes surface only inside generated output.
