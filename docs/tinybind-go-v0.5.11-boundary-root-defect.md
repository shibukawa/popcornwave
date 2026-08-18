# Defect report: hoisting a value binding removes a component's boundary root

**From:** Popcorn Web (`github.com/shibukawa/popcornweb`)
**Against:** `github.com/shibukawa/tinybind-go` v0.5.11
**Date:** 2026-08-14
**Status:** open — adoption held at v0.5.10

## Summary

`decision:value-binding-hoisting` collects a block's bindings to the front and
nests everything else into the innermost body. `boundaryRoot` walks a
component's body list looking for exactly one `*ElementNode` and returns nil on
any node kind it does not recognise. A `ValNode` is one of those.

So after hoisting, **a component whose body holds any `val` binding presents a
single `ValNode` where its root element used to be**, and every caller of
`boundaryRoot` reads that as "no root".

Two callers, and they fail differently:

| Caller | Symptom |
| --- | --- |
| `emit.go` script-block check | generation error, wrong but loud |
| `boundaryCandidate` → update boundary | **no error; the component silently stops being an update boundary** |

The second is the serious one, and it lands on exactly the shape v0.5.11 now
requires: the typed page rung is gone, so a discovered page that loads its own
data *must* use `val`, and every such page quietly loses its boundary.

## Reproduction

Both were measured on our page-tree fixture, moving it from the retired typed
rung to the explicit shape your diagnostic recommends.

**Silent boundary loss.** A page component with a top-level `val` binding:

```html
export component Page(id: string, page: int?): html {
  <section>
    {val view = LoadUser(id, page)}
    <h1>{view.name}</h1>
  </section>
}
```

Generated output loses all three of these, with no diagnostic:

```
- var planPageBoundary = &htmlbind.Boundary[PageParams]{ ... }
- 	Boundary:    planPageBoundary,
- 	planPageOps.BoundaryAttr(),
```

**Loud but wrong.** The same component with a `<script component>` block fails
generation:

```
page.pw.html:10:1: component Page declares a script block and must render
exactly one root element, because the marker naming its declaration lives on
that element
```

It renders exactly one root element. Removing the `val` compiles; removing the
script block compiles; the two together do not.

## Cause

`templates/htmlbind/boundary.go`:

```go
func boundaryRoot(nodes []Node) *ElementNode {
	var root *ElementNode
	for _, node := range nodes {
		switch node := node.(type) {
		case *TextNode:      // whitespace tolerated
		case *CommentNode, *DoctypeNode, *HeadNode:
		case *ElementNode:   // at most one
			root = node
		default:
			return nil       // ← a ValNode arrives here
		}
	}
	return root
}
```

`boundaryCandidate` (line 73) and the script-block check in `emit.go` (line 208)
both call it.

Before hoisting this could not happen: normalization split a body at the
binding's position, so a binding written after the root element left that
element a sibling. Hoisting moves the binding in front of it, which puts the
root element inside the binding's body.

## Suggested fix

`boundaryRoot` descends through a `ValNode` into its body rather than refusing
it. A binding contributes no output, so the root element of the body is the root
element of the component — which is the same reasoning that already lets the
function step over comments, doctypes and head nodes.

We have not sent a patch because the recursion has a choice we should not make
for you: whether a `val` nested several deep still yields a root, and whether
two sibling bindings after normalization (which nest) behave the same as one.

## Why we held adoption

We cannot take v0.5.11 in either direction:

- staying on the typed rung fails generation, since v0.5.11 removed it;
- moving to `val` compiles and silently drops the update boundary from every
  page that loads its own data, which is a working render with a degraded delta
  path and no diagnostic.

That second sentence is your own phrase, from `requirement:template-value-binding`
`sequence_splice.failure_if_missed`, written about a different code path. The
same hazard reached this one.

We are on v0.5.10 and green. We will adopt v0.5.11 as soon as `boundaryRoot`
sees through the binding, and the migration off the typed rung is written and
waiting behind it.

## Not part of this report

Everything else we have exercised in v0.5.11 behaves as its knowledge describes.
The failing-external rule, the hoist reaching a status before the first byte,
and the settled-boundary cache are all what we asked for and we have no findings
against them — the hoist is what exposed this, not what is wrong with it.
