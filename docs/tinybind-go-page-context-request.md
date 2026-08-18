# Change request: let a page reach the request context

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.3.5
**Date:** 2026-08-05
**Status:** not yet raised

## Summary

A page under `routetree` cannot reach the context of the request that is
rendering it. Three separate things add up to that, and only the third is a
design question — the first two look like oversights, and one of them is an
inconsistency with behaviour the same release already ships elsewhere.

1. **`routetree` never computes `ContextExternals`.** The identical `external`
   declaration receives a context in a `templates/` package and does not in a
   route package.
2. **A typed `Load` cannot declare a leading `context.Context`.** Every
   parameter is validated as a URL-bindable scalar, so the only escape is the
   handler rung.
3. **`external async` is excluded from context injection** for a stated reason
   that does not hold for `async` — only `live` receives a context through its
   own call shape.

Together these mean that a page needing anything that lives on the request
context — a database pool, an authenticated session, a request-scoped tracer —
must take the handler rung and give up the typed page entirely.

A fourth ask is unrelated and much smaller, and it is what makes the third
consequence bite: **a tree whose every route is on the handler rung does not
compile**, because the registry imports an error package it never uses. So the
workaround for the first three has a bug of its own waiting at the end of it.

## Why this matters downstream

Popcorn Web puts the database handle and the authenticated session on the
request context. That is our design, not the module's, and we are not asking the
module to know about either. What we need is the context itself; everything on
it is ours to look up.

The concrete cost showed up while writing a tutorial chapter. The chapter builds
a page listing the signed-in reader's records — the most ordinary page a website
has — and it cannot use the typed rung the documentation presents as the normal
one. It has to open with:

> This page needs `auth.User` and a database pool, and both arrive on the
> request context, so it takes the rung above: `func Load(w, r)` generates the
> registration and leaves the response to you.

That sentence is accurate and it is teaching a workaround. The typed rung is the
better shape — generation checks the result list against the component's
parameter list, and the handler rung checks nothing — and today it is available
only to pages whose entire input is in the URL.

## Ask 1 — pass `ContextExternals` when compiling a page template

**This is an inconsistency rather than a missing feature.** The mechanism exists,
works, and is already applied to one of the two paths that compile a `.pw.html`.

`generator/templates.go:462 contextExternals(dir)` scans the `.go` files beside a
template for functions whose first parameter is `context.Context`, and
`generator/templates.go:364` and `generator/artifacts.go:214` pass the result
into `htmlbind.GenerateOptions.ContextExternals`.

`routetree/generate.go:197 compileTemplate` builds its `htmlbind.GenerateOptions`
without it:

```go
return htmlbind.Generate(path, source, htmlbind.GenerateOptions{
	Package:              pkg,
	ServerActions:        actionURLs(actions),
	ServerActionResolver: resolver,
	ServerActionAttr:     emitter.ActionAttr,
})
```

### Reproduction

The same declaration, in the two places.

In `templates/probe.pw.html` with `templates/probe.go`:

```html
package templates

external CurrentToken(): string

export component Probe(): html {
  <p>{CurrentToken()}</p>
}
```

```go
package templates

import "context"

func CurrentToken(ctx context.Context) string { return "t" }
```

generates, and compiles:

```go
planProbeOps.TextCtx(func(ctx context.Context, p ProbeParams) string { return CurrentToken(ctx) }),
```

The same pair moved into `pages/archive/` generates:

```go
func(p PageParams) []Memo { return RecentMemos() },
```

which fails to build:

```
pages/archive/page_pw_gen.go:31:39: not enough arguments in call to RecentMemos
	have ()
	want (context.Context)
```

### Proposed shape

Call `contextExternals` for the route package directory and pass it through, the
way `templateArtifacts` already does. The scan is syntactic and skips a file that
does not parse, so it is safe to run before the package compiles — which is the
property `routetree` needs.

If the scan belongs to `generator` rather than to `routetree`, exporting it, or
adding a `ContextExternals` field to `routetree.GenerateOptions` for the caller
to fill, both work for us. We have no preference about which side owns the walk.

## Ask 2 — let a typed `Load` declare a leading `context.Context`

`routetree/pagefunc.go` classifies the entry point by signature and validates
every parameter as URL-bindable:

```go
for i, param := range fn.Params {
	_, optional, ok := bindableType(param.Type)
	if !ok {
		kind := "query parameter"
		if i < len(route.Params) {
			kind = "path parameter"
		}
		fail("%s %q has type %s; a page input must be a scalar the decoder can bind from a URL",
			kind, param.Name, param.Type)
		continue
	}
	...
}
```

`routetree/registry.go pageBinding` then maps every parameter positionally onto
the decoded route:

```go
args := make([]string, len(analysis.Page.Params))
for i, param := range analysis.Page.Params {
	args[i] = "route." + ExportedName(param.Name)
}
```

So `func Load(ctx context.Context) ([]Memo, error)` is rejected at generation:

```
pw: pages: pages/archive/page.go:5: query parameter "ctx" has type context.Context;
a page input must be a scalar the decoder can bind from a URL
```

The diagnostic is good — it explains the rule it is enforcing — and the rule is
right about *URL inputs*. The context is not a URL input. It arrives from the
request rather than from the address, which is exactly why it cannot be
expressed today.

### Proposed shape

Recognise a leading `context.Context` and exclude it from the input list.

The detection can stay syntactic in the style `isHandlerSignature` already uses
— that function resolves the `net/http` import name from the file rather than
from type information, and `importName` is already there to do the same for
`context`. Concretely:

- `PageFunc` gains a flag, say `TakesContext bool`, set when
  `params[0]` is the file's spelling of `context.Context`;
- `InspectLogic` trims that parameter out of `PageFunc.Params`, so `Validate`
  needs no change at all — the route-order match and the bindable check both go
  on reading the list they already read;
- `pageBinding` prefixes `args` with the request's context when the flag is set.

We think the trim is the version worth having, because it keeps the rule
"`Params` are the URL inputs, in route order" true rather than adding an offset
that every reader of that code then has to carry.

The position is worth fixing rather than accepting anywhere: leading only, which
matches Go's own convention and keeps the route-order rule for everything after
it.

### Why not just document the handler rung

That is what we do today, and it works. What it costs is the check: at
`RungTypedPage` generation compares the function's result list against the
component's parameter list and fails naming both. At `RungHandlerPage` nothing
is compared, because the response is the caller's. So the page that most needs
the check — one assembling several values for a template — is the one that
cannot have it, purely because it also needs a pool.

## Ask 3 — reconsider excluding `external async` from context injection

`templates/htmlbind/emit.go:43` states the exclusion and its reason:

```go
// takesRenderContext reports whether a synchronous external's Go implementation
// declared a leading context.Context. An async or live external is excluded:
// those already receive the boundary context through their own call shape, and
// they can only be called in an await binding, where ctx is in scope anyway.
func (e *goEmitter) takesRenderContext(name string) bool {
	if !e.contextExternals[name] {
		return false
	}
	signature, ok := e.c.externals[name]
	return ok && !signature.async && !signature.live
}
```

The reason holds for `live`. We verified that: an `external live` receives a
context through `LiveBinding`, in a route package, with no `ContextExternals`
involved — which is why Ask 1 does not affect it.

We could not reproduce it for `async`. Declaring

```html
external async RecentMemos(): Memo[]
```

against `func RecentMemos(ctx context.Context) ([]Memo, error)` generates

```go
func() error { value, err := RecentMemos(); scope.Memos = value; return err },
```

which fails with `not enough arguments in call to RecentMemos`. The closure the
call sits inside does have a `ctx` in scope, which matches the second half of the
comment — the value simply is not passed.

This may be deliberate and merely described together with `live` for brevity. If
so, we would like to understand the intent, because from the outside the two
halves of the comment describe `live` and the code covers both.

If it is not deliberate, the fix is dropping `!signature.async` from the
condition, and `async` then opts in the same way `sync` does — by the Go function
declaring the parameter.

## Ask 4 — do not import the error package into a registry that never uses it

Unrelated to the context, found while writing the same chapter, and much
smaller: a page tree whose every route is on the handler rung produces a
registry that does not compile.

`routetree` emits the configured error import into `routes_pw_gen.go`
unconditionally, but the error block it belongs to is written only for the
decoders of the template-only and typed rungs. A tree with neither has the
import and no use of it.

### Reproduction

A tree with one route, `pages/page.pw.html` beside a `page.go` declaring
`func Load(w http.ResponseWriter, r *http.Request)`:

```
pages/routes_pw_gen.go:6:2: "github.com/shibukawa/popcornweb/pw" imported and not used
```

Adding any second route on another rung fixes it, which is what makes this easy
to miss: the scaffolded tree has a template-only root, so the shape only appears
once somebody removes it. Ours did — a project whose page tree exists alongside
an older handler package has no use for a root page, and deleting it is the
natural thing to do.

### Proposed shape

Emit the error import only when the file uses it. The emitter already knows the
rung of every route in the tree by the time it writes the registry, so the
condition is whether any of them is not the handler rung.

## What we are not asking for

- **Not a request object.** `*http.Request` in a typed `Load` would pull the
  transport into a signature whose whole point is that it has none. A
  `context.Context` is the smallest thing that carries what we need.
- **Not knowledge of what is on the context.** Pools, sessions, and tracers are
  ours. The module needs to pass the value through, not to look inside it.
- **Not a change to the handler rung.** It is the right escape hatch and we will
  go on using it for pages that genuinely own their response.
- **Not a change to the URL-binding rule.** Ask 2 exempts one parameter that is
  not a URL input; every remaining parameter should stay a bindable scalar, and
  the current diagnostic for one that is not should stay exactly as it is.

## What we do meanwhile

Pages needing the request context take `func Load(w, r)`, and our tutorial says
why. Ask 1 is the one that would change the most for the least, because it makes
one already-shipping behaviour consistent across the two compile paths — a page
could then reach the context through a synchronous `external` without any new
concept, and Ask 2 would become a convenience rather than the only way.
