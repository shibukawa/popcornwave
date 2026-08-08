# Partial updates

Two routes under one layout, a search form, and a table whose rows are
addressable on their own. Nothing here is about how the page looks — it is about
what leaves the server when you click.

```bash
pw dev
open http://localhost:8080/
```

`pw dev` is the whole command, and the reason is worth one sentence: the served
static tree is derived from the authored one — hashed names, compressed variants
— so `public.go` embeds `dist/public` rather than `public`, and a checkout that
has never been built has nothing there to embed. Running the binary directly
works after `pw build`:

```bash
pw build && APP_ENV=dev go run ./cmd/partial_update
```

Open the network tab before you click anything. Compression is off in
`config.dev.toml` so the sizes are the responses rather than their compressed
form, and every response is reproducible: the data is a fixed table, so the same
query always produces the same bytes.

## What to click, and what to look for

**Follow the `Invoices` link.** One request, `application/x-ndjson`, about a
kilobyte. The document it replaces is five. The layout — the header, the nav,
the footer — is not in it, because a layout is a chain member and therefore an
update boundary, and the browser told the server which version it holds.

**Search for `e`, then for `a`.** The interesting one. The response says:

```
  248 B  replace    c2       the form, whose input value changed
  221 B  children   orders   the table's own markup is unchanged; its rows are now these, in this order
   89 B  unchanged  c1       the layout
   75 B  unchanged  o-1041   a row that survived the search, ×9
```

The table is never re-sent. Nine rows that appear in both results are never
re-sent. What travels is the new arrangement — a list of ids — and the form
field whose value changed.

That is why the search field is **outside** the table in `page.pw.html`. An
input carrying the query is markup that changes with the query, and a boundary
containing it could never compare equal. Move it inside and the children
operation disappears; that edit is worth making once, to watch it happen.

**Press a row's `redraw` button.** One request for one row, and the row flashes.
The parameters travel from the DOM, because a redraw's arguments come from
whoever asked for it — which is also why a component that loaded a record by
identifier would have to check ownership itself. This one formats values handed
to it, which is the shape that is safe to publish without a check.

**Watch the console.** `public/redraw.js` subscribes to the runtime's events, so
every navigation, redraw, and fallback is logged with its reason.

## What is in the records

A fragment does not travel as markup. It travels as the address of its static
shape and the values that fill it:

```json
{"r":"op","kind":"replace","id":"o-1041","frame":"…","parent":"orders",
 "seq":"90UUJgsc-c5m7KmQih3SBA","values":["…","o-1041","Espresso machine","alice","2026-08-01"]}
```

Every row shares one `seq`, because they are one component. The tree behind that
address is fetched once:

```
GET /
Pw-Render: sequence
Pw-Sequence-Address: 90UUJgsc-c5m7KmQih3SBA

Cache-Control: public, max-age=31536000, immutable
```

It derives from the template rather than from the request, which is the one
thing on this wire that is not per user — so it is public, and a shared cache
may hold it across users until a template changes.

## Turning it off

Set `enabled = false` under `[html.update]` in `config.dev.toml` and reload.
Every link is an ordinary navigation, every search is an ordinary form
submission, and the pages are identical. That is the property worth checking:
the feature is a transport, and removing it removes nothing a reader can see.

## What this example does not show

A live region, which [`../live_render`](../live_render) covers; an action
response, which is `pw.WriteUpdate` from a POST handler; and the cost of any of
this, which `go test ./pw -run TransferCost -v` in the repository root reports
as a table.
