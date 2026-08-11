# Change requests: what a scoped script still needs

> **Superseded 2026-08-11.** Item 2 shipped in v0.5.6 and is integrated here.
> Item 1 was confirmed and turned out to need an identity decision first;
> `docs/tinybind-go-scope-identity-reply.md` is our answer and the live thread.
> This page is kept for the reasoning that produced both.

Found against **tinybind-go v0.5.5** on 2026-08-11, while wiring
`requirement:component-script-block` and `requirement:scoped-script-declaration`
through Popcorn Wave. Two gaps, independent of each other, and together they are
what stands between the shipped module half and a working feature.

---

# 1. The client cannot join an asset to an instance

`requirement:scoped-script-declaration` rests on this, and the wire contract
states it outright:

> A **named** `Scope` … is the same identity the manifest already carries as
> `component_id` on every instance. Joining an asset to a live region therefore
> needs no second identity scheme and nothing new on the wire.

That is true of the design and not of the shipped wire. Checked three ways:

- The manifest entry format in the same document is
  `<instanceId>:<frame>[:<children>[:<parent>]]` — no component identity.
- `htmlbind.Builder.BoundaryAttr` writes one attribute, the instance id. A
  rendered instance therefore carries `data-<P>-id` and nothing naming its
  declaration.
- `grep -rn 'ComponentID\|component_id'` over `htmlupdate/` and
  `internal/updatecore/` finds nothing.

`data:component-update-manifest` does list `component_id` — as one field of a
design that has not all shipped. A client holding `Scope: "Counter"` has no way
to find the elements that are Counters.

**What we are asking for:** the declaration identity reaching the client for
each live instance, by whichever route is cheaper on your side —

- a fifth manifest field, `<instanceId>:<frame>:<children>:<parent>:<componentId>`,
  which fits the existing colon grammar and the documented skip-a-malformed-entry
  rule; or
- a second attribute beside the instance id, which costs bytes per instance but
  needs no manifest change and survives a client that never received a delta.

The first is enough for us. We do not need both.

**What we shipped without it.** A chain member — the document, a layout, the
page — has exactly one instance per render, so its position in the composition
identifies it as precisely as an instance id would. We compute the chain from
each layer's own `Assets()`, send it outermost-first on the document marker and
on a navigation delta header, and diff it on the client: a common prefix stays
mounted, the tail below releases innermost-first and remounts outermost-first.
That covers a page's own script, which is the case this started from.

**What is still blocked** is the rest of the axis you chose, and it is the more
general half: a scoped script on a component nested inside a page, which can
have many instances. Nothing distinguishes them, so nothing can start or release
per instance.

---

# 2. A page tree needs a channel for its extracted assets

A page tree compiles its templates through `routetree.Generate`, which returns
`[]routetree.Generated`:

```go
// Generated is one emitted file and where it belongs.
type Generated struct {
	// Path is the absolute destination.
	Path string
	// Source is the formatted Go source.
	Source []byte
}
```

Go source and nothing else. So when a page or layout declares a
`<script component>` block:

- the compiler extracts it and computes its URL;
- the generated component records that URL, correctly —
  `Assets: []htmlbind.Asset{{ID: "page.script.23e0bc188fe5", Type: "text/javascript", URL: "/public/generated/page.script.23e0bc188fe5.js", Scope: "Page"}}`;
- and **the bytes are dropped**, because nothing returns them.

The result is a page that references a module which answers 404. Nothing
downstream can repair it: the content only ever existed inside the compile.

Verified by adding a script block to a page in our fixture tree and
regenerating. The asset literal appears in the generated Go with the right URL
and hash; no file is written anywhere in the tree.

## Why the flat path is fine

`generator.GenerateArtifacts` already returns extracted assets as artifacts with
`Destination: DestinationPublicAsset`, an `OutputBase`, an `Extension`, and the
`Content`. We now write those, and a component script block works end to end for
templates compiled that way. This request is only about the page tree.

## What we are asking for

A way for `routetree.Generate` to return the non-Go files a tree's templates
produced. The smallest shape that would do it:

```go
type Generated struct {
	Path   string
	Source []byte
	// Content is the verbatim bytes for a file that is not Go, with Source
	// empty. A public asset extracted from a component's style or script block
	// arrives this way.
	Content []byte
	Kind    GeneratedKind   // go_source | public_asset
}
```

Any equivalent works — a second return value, a separate accessor, a callback.
What matters is that the bytes reach the caller with enough to know where they
go. We already resolve the destination directory ourselves for the flat path, so
a URL or a base name plus an extension is sufficient; we do not need
`routetree` to write anything.

## Related, and probably the same fix

`routetree` compiles a page tree's templates without threading the compile
options through. That is already recorded on our side as why a page tree takes
the module's default boundary prefix while a registered-router template can take
a configured one — so a page tree and a flat template disagree about a setting
inside one project. Both symptoms come from the same place, and a change that
lets the tree run carry the caller's options would close them together.

Concretely, `routetree.Config` carries `Root`, `ImportBase`, and the three file
names, and nothing about assets or prefixes.

## Impact if it stays open

`requirement:component-script-block` is usable for components compiled on the
flat path, and unusable for a page or a layout in a page tree — which is where a
page's own script most obviously belongs. Since a page tree is the routing style
we scaffold by default, that is most projects.

---

# What we already did on our side

Neither of the above is the whole distance. The rest was ours and is done:

- Extraction runs with no configuration — the generator defaults write under
  `public/generated`, which is where our public tree already looks — so the only
  thing between a declared block and a working asset on the flat path was our
  own artifact filter not naming `ArtifactStylesheet` and `ArtifactScript`. It
  dropped them silently, so the generated component recorded an asset URL and no
  file was ever written.
- The two public-asset shapes now route correctly: a conversion owns its naming
  and stages outside the served tree, while an extracted block is a base plus an
  extension and belongs in the public tree its URL was computed against.
- The signal channel, its client table, and the reserved-prefix guards on both
  backends were already shipped against v0.5.3.
