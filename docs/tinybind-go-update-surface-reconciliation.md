# The update surface, checked against v0.4.4

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** the usage guide of 2026-08-09, and `github.com/shibukawa/tinybind-go` v0.4.4 as published
**Status:** reconciliation. What the guide describes, what v0.4.4 does, and where the two differ.

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

## Two questions back

**Should `Options.Render` take options, or should the guide say it does not?** Either resolves it. We would take the variadic, since the rule "pass the same ones everywhere" is worth having no exception to.

**Is the sequence address header a rename or a prefix change?** The guide's table reads as the former. If it is, `Sequence` reads `<prefix>-Sequence` today and would need to read `<prefix>-Sequence-Address`.
