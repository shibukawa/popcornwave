# The update surface, checked against v0.4.4

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** the usage guide of 2026-08-09, and `github.com/shibukawa/tinybind-go` v0.4.4 as published
**Status:** every item is closed. §1 and §2 in v0.4.5, §3 and the follow-up in v0.4.6. Kept as the record of the round, and of one defect it found on this side.

## Summary

We read the guide against the published module and wired our runtime to it. **Most of the guide is v0.4.4 exactly**, including the parts we had not yet found: `Options.Render`, `Registry.MustRegister`, `Registry.RequiredHead`, the four-field manifest, and the media-type discriminator are all there and behave as written.

Four differences, in descending order of what they cost:

1. **The `children` operation does not reach a streamed navigation.** It is produced and then flattened by the stream writer. This is a defect rather than a naming gap, and it disables the feature the guide's own example is about. Reproduction below.
2. **The header names are the guide's, not v0.4.4's**, which the covering note says. One of them is a rename rather than a prefix change and needs stating separately.
3. **`Options.Render` takes no render options**, where the guide says every entry that renders a fragment does.
4. **A `values` array's contents are not what the guide's example shows** — or rather, they are, and the example is worth keeping because it is the part a client implementer gets wrong.

Everything we changed on our side is listed at the end.

## 1. The children operation is dropped on the streamed path

The guide describes it as the answer to a list that gained a row:

> **`children`** — the region's own markup is unchanged and its nested boundaries are now these, in this order. No markup at all.

It is produced. `delta.operations` emits `OpChildren` when a boundary's frame validator matches and its children validator does not, and both the buffered and the streamed paths call the same function.

It is then lost. `renderStream` dispatches on whether an operation carries markup rather than on its kind:

```go
case item.Operation != nil && item.Operation.HTML != "":
    → Replace / ReplaceValues        // carries kind and boundaries
case item.Operation != nil && live:
    → nothing
case item.Operation != nil:
    → Unchanged(id, frame)           // drops kind and boundaries
```

A `children` operation carries no markup, so it takes the third branch and arrives as a bare validator restatement. `DeltaStream` also has no `Children` writer, so there is nothing the branch could have called.

**Reproduced**, on a three-row list gaining one row, with the page's own markup unchanged:

```
{"r":"head","build":"b1"}
{"r":"op","id":"the-list","frame":"lo0Siz…"}          ← should be kind children, with boundaries
{"r":"op","kind":"replace","id":"row-3","html":"…"}   ← the new row, with nowhere to go
{"r":"end","reason":"final"}
```

The list's markup is unchanged, so no hole for `row-3` exists anywhere on screen. Our client cannot place the operation and falls back to an ordinary navigation — correct, and a full page load to add one row.

The buffered path is unaffected: `operationBody` copies `Kind` and `Boundaries`. So this is the stream writer specifically, and it is the only path we use — a page under our page tree renders through `RenderStreamAsync`.

**The fix looks like one branch and one writer.** We have implemented the client half against the shape the guide documents, so it starts working the moment the record arrives.

## 2. The header names

The covering note says the header changes are not in v0.4.4, and they are not. `DefaultHeaderPrefix` is already `X-Tinybind`, so the prefix in the guide's table is not the difference. One name is:

| guide | v0.4.4 |
| --- | --- |
| `X-Tinybind-Sequence-Address` | `<prefix>-Sequence` |

The rest compose from `HeaderPrefix` exactly as the table says. We configure ours to `Pw`, so nothing here reaches us as a name; we are noting it because the guide reads as a rename rather than a prefix change, and a caller who copied the table would ask for a sequence at a header the module does not read.

## 3. `Options.Render` takes no options

The guide is emphatic, and we agree with it:

> Every entry that renders a fragment takes `[]htmlbind.Option`. **Pass the same ones everywhere.**

`Render` is the exception:

```go
func (o Options) Render(w http.ResponseWriter, r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) error
```

The buffered entry is the one a page with no `await` boundary would use, which is the ordinary small page — so the entry most likely to be reached first is the one that cannot be given a CSRF token. A component with an unsafe form on such a page fails to render, and the guide's own warning about that failure cannot be acted on from this entry.

We do not hit it, because we render through the streaming entry. We are reporting it because the rule the guide states is the right rule and this is the one place the surface does not hold it.

## 4. The manifest is four fields, and we were sending two

Not a divergence — a correction to us. `EncodeManifest` and `DecodeManifest` carry `id:frame:children:parent`, with the trailing fields omitted when empty and the children field written as empty when only a parent follows.

Our client held only the frame. The consequence is not a broken screen but a wasted one: with no children validator returned, every parent boundary's arrangement compares unequal, so the server states a `children` operation on every navigation for every boundary that has nested ones. Fixed, with the encoder's omission rule mirrored and covered in our conformance harness.

This is also what made the reproduction in §1 legible: the record we saw was a `children` operation that had lost its kind, not an unchanged boundary.

## 5. The `values` example is right, and worth keeping

```json
{"kind":"replace","id":"panel","seq":"Yb3_x…","values":["Inbox","30","data-tb-id","r0", …]}
```

`data-tb-id` and `r0` in a value array look like a mistake until you read `componentNode`: a component call that opened a boundary contributes a placeholder frame whose two varying parts are the attribute name and the id. So a value stream is not a list of interpolations — it interleaves interpolations, branch markers, repeat counts, and the two halves of a boundary's placeholder attribute.

We mention it because it is the thing a client implementer will get wrong: the walk has to consume one value per hole, one per conditional, one per loop, **and one per component call**, in instruction order, or every subsequent value lands in the wrong place with no error to say so. `Sequence.Reassemble` on the server is the reference and we intend to test our walk against it rather than against our reading of it.

## What we implemented against v0.4.4

- **Render options on the action path.** This is the 500 we reported: `WriteUpdate` with a region holding a form answered a problem document where the documented behaviour is 422 carrying the validation errors. It is fixed by the variadic and by passing the token.
- **Holes, and retain versus install.** A hole whose boundary we hold takes that live node, moved rather than re-rendered. A hole we do not hold is left for the operation that fills it. We decide by what we hold rather than by which operations the response carries, because on the streamed path a parent lands before its children's records exist — there is nothing else to consult, and retaining a node an operation then replaces costs one swap and converges.
- **The `children` operation**, implemented and currently unreachable for the reason in §1.
- **The redraw body as `ops`, `head`, and `manifest`.** The base64 head header is retired on our side and the returned validator is now held, so a redrawn region is no longer re-sent by the next navigation.
- **Obligations 3 and 4.** Pending redraws abort and the outgoing live connection closes before navigation records apply; the new live connection opens after the last record lands.
- **The four-field manifest**, per §4.
- **The sequence mode**, mounted on the page's own URL ahead of the redraw branch, answering from a lookup with no render behind it.

## What we have not implemented

- **Walking sequences.** We send no `-Sequences` header, so we are served assembled markup and everything works. The remaining work is the fetch-and-cache by address and the tree walk. Staged deliberately: it is an optimisation over markup that is always available, and a wrong walk is silent.
- **Moving our live delivery body onto this grammar.** Our live records carry a per-delivery validator that suppresses a re-sent unchanged boundary, and the module's live mode writes every completion. Converging costs us that suppression until a completion can carry a validator, which is the open item from the previous round.

## Fixed on main, and verified

`584af8e` on `main` closes §1 and §2. We re-ran the reproduction against it, and the record that was a bare validator restatement is now the operation:

```
{"r":"op","kind":"children","id":"the-list","frame":"lo0Siz…",
 "boundaries":["row-0","row-1","row-2","row-3"],"children":"TXgVUu…"}
{"r":"op","kind":"replace","id":"row-3","html":"…"}
```

Three things in that fix are worth naming back, because two of them we had not found.

**The buffered path had the same defect**, and worse. `renderDelta` also dispatched by whether markup was present, so a children operation was written as a `replace` carrying no HTML — which does not fail to reorder a region, it **empties** it. We reported the streamed path because it is the one we use; the fix found the other half.

**The record gained a `children` field**, on every operation including an unchanged one, so a manifest rebuilt from a stream returns both halves. That is the counterpart to the four-field manifest in §4 and it closes the loop: without it, a client that navigated through a stream would return frames only and make every list compare reordered — the same waste, arriving from the other direction.

**`Unchanged` grew a parameter** rather than gaining an overload, so nothing can call it and silently drop the validator.

We have taken all of it: the client records `children` from stream operation records, and its conformance harness covers a replaced boundary and an unchanged one both returning it.

### One thing the fix does not carry: `parent`

A stream operation record has `frame` and now `children`, and no `parent`. The buffered manifest array has all three.

`disappeared` reads `known.Instances[].ParentID`: a boundary the client held that this render does not produce, whose parent it cannot name, forces a replacement of the outermost boundary. So a client whose manifest came from a stream falls back to a root replacement whenever a list **shrinks**, where one that came from a buffered response would not.

It is the conservative direction — correct, and expensive exactly where the children operation is cheap. It is also the same shape as the `children` field you just added, which is why we mention it rather than filing it separately: a stream record carrying two of the three fields a manifest entry has is a client that rebuilds two thirds of its state.

## Closed in v0.4.6

**`Options.Render` takes render options**, so the rule the guide states now has no exception.

**A stream operation record carries `parent`**, alongside `frame` and `children`, gathered into a `ManifestEntry` so the three travel together and a writer cannot add a field without every call site seeing it. A manifest rebuilt from a stream is now the whole entry, which closes the last of the three ways a client could return two thirds of its state.

We have taken both, and the sequence walk is implemented against them.

## The sequence walk, and what building it found

We implemented the walk and tested it the way we said we would: against `Sequence.Reassemble` rather than against our reading of the specification. `pw/sequencefixture_test.go` renders three shapes — an empty conditional branch, a one-row loop, and a three-row loop with an optional attribute carrying characters the escaper touches — takes the address and values the module published, and asserts the module reassembles its own split back into the bytes it rendered. The tree, the values, and that expected markup are committed, and the browser harness drives them end to end: navigation record carrying `seq` and `values`, sequence response carrying the tree, and the installed markup compared against what the server produced.

It caught nothing about the walk, which is the answer we wanted. It caught something about us.

**Our conformance harness had stopped checking anything.** The verdict — the `if (failures) process.exit(1)` and the success line — sat mid-file, and every case we appended while following v0.4.4 landed after it. Those cases ran, counted their failures into a number nobody read, and reported success that had already been printed. We found it by mutating the walk to iterate a loop one time too few and watching the suite pass.

Fixed by moving the verdict to the end of the file, with a comment saying it has to stay there. The same mutation now fails four checks, on exactly the two fixtures that carry a loop.

We mention it because it is the same class as the defect this round started with. A `children` operation dispatched by whether markup was present, and a test suite whose verdict was dispatched by where it happened to sit — both are correct-looking code whose failure mode is silence, and neither is visible in a diff.

## Two questions back, both answered

**Should `Options.Render` take options?** Answered: it does, in v0.4.6.

**Is the sequence address header a rename or a prefix change?** Answered by the fix: a rename, to `<prefix>-Sequence-Address`, with the reason stated in the source — a pair reading `-Sequences` and `-Sequence` is two headers a reader tells apart by counting characters.

## One practical note

We are on `v0.4.6`. Nothing in this document is outstanding on either side.
