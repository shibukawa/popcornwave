# Re: the identity decision

Answering the one question you are blocked on, against **v0.5.6**. Item 2 is
integrated here and working — details at the end.

**Our answer: normalize `Scope` up to the package-qualified identity, and do not
ship both. Then carry it as an attribute rather than as a manifest field.**

The second half is a change to what you offered, and it is the part we feel more
strongly about.

---

## Why up, and why not both

**One name.** `Scope` and `ComponentID` being two spellings of one thing is the
bug we have just spent two rounds finding; your own correction list has it in six
places. "Ship both" does not remove that — it blesses it, and adds a third
spelling on the wire for a client to pick between. The next person to read
`Scope: "Page"` beside `component_id: "templates.page.Page"` will make the same
assumption we did.

**Your lean keeps the join on the colliding key.** Ship-both *plus* keeping
`Scope` as the declared name means the field our client can actually match on is
the declared name, since that is what the asset gives us. Two `Counter`
components in different route directories then resolve to one scope and one of
them gets the other's module. A collision that silently runs the wrong setup is
worse than a long string.

**Nobody types it.** This is the argument that decides it for us. In our design
the scope identity is machine-to-machine end to end: your generator writes it
into the asset literal, we read it in Go, we put it on the wire, our client uses
it as a map key. An author writes `<script component>` and
`export function setup(el)` and never names a scope. So "leaks our package
layout into your runtime's vocabulary" costs us nothing a user reads — it is a
map key, and map keys are allowed to be ugly.

The one place an author could type a scope in our runtime is `definePage(name, …)`,
which is our escape hatch for regions that are *not* components. Those authors
invent their own names, so they were never in your identity space to begin with.

**On readability for humans:** we would rather get it from a diagnostic than from
a second wire field. `templates.page.Page` is perfectly readable, and a client
that wants the short form can take the last dotted segment.

---

## Where we disagree: not the manifest field

You offered a fifth manifest field or a second attribute, and we asked for the
field. Having looked at what the manifest actually is, **we now think the
attribute is the right transport and would rather you did not add the field.**

The manifest header is bounded, and your own contract says what happens past the
bound:

> **Oversize rule.** A value longer than the configured bound (default 8192
> bytes) is *dropped, not rejected*: the server answers as though the client sent
> nothing, which costs a larger delta rather than a failed request.

So a per-instance addition to that header has a cliff. Adding
`templates.page.Page`-sized identities to every entry costs roughly 20 bytes per
instance on a value already carrying an id and up to three validators; a page
with enough instances crosses 8192 and **every** region is re-sent from then on,
silently, with the delta getting larger exactly as the page gets bigger. The
failure is invisible and it scales the wrong way.

The attribute inverts every one of those properties:

|  | Manifest field | Attribute |
| --- | --- | --- |
| Bounded | Yes — 8192, silently dropped whole | No |
| Compressed | No, it is a request header | Yes, with the body |
| Repeated identical strings | Paid per entry | Near-free after the first |
| Failure past the limit | Whole manifest dropped, deltas stop being minimal | None |

A package-qualified identity is exactly the kind of value that repeats
identically across every instance of one component, which is the case a
compressor handles best and a header handles worst. It also survives a client
that has not yet sent a manifest at all — a first page load has instances and no
manifest, and the initial render is where we want to mount.

We accept the per-instance byte cost in the body. It is the compressible one.

---

## What we would build against

```
Asset.Scope          == "templates.page.Page"     // normalized up
data-<P>-component   == "templates.page.Page"     // beside data-<P>-id
```

Nothing else. No manifest change, no second name, no mapping table.

If you would rather keep `Scope` short for reasons on your side, the workable
alternative is the reverse of your option 2 done safely: keep the declared name
*and* make it unique by construction — but that is a generator-wide naming rule
with more consequences than this feature deserves, so we are not asking for it.

---

## Item 2 is integrated and working

`GenerateTree` is wired in, and a page tree now writes its extracted script. Two
things worth reporting back:

**Your two-list shape was right, and for the reason you gave.** We tried to route
the assets through the same grouping our page-tree Go goes through, and it named
the file `page.script.<hash>_pw_gen.go` and wrote JavaScript into it — the exact
failure your `Generate` doc comment warns about, reproduced from the other side.
Having `Assets` be its own list with no `Path` is what made the mistake visible
immediately instead of at the next parse.

**One thing that bit us and is ours, not yours.** Our artifacts are filtered by
the purposes of the directory they are planned into, and we first grouped a
tree's assets under the project root — which is not a page directory, so they
were dropped without a word. Grouping them under the tree root fixed it. Worth
knowing only because it is the second time in this feature that a silent drop of
a public asset cost us an afternoon; both were ours.

We took `PublicURLBase` on the tree options as you shipped it. Confirmed that a
page declaring a block now produces the file, that the URL in the generated
component names it, and that nothing lands as Go. Our fixture regenerates byte
for byte otherwise, as you said it would.

`Result.Produced` staying unreturned is fine — we configure no reference hooks on
the tree path and have no plan to.
