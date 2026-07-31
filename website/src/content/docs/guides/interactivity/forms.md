---
title: Forms
description: Client-side feedback that agrees with server-side checks, dialogs that submit, and suggestion lists that need no script.
sidebar:
  order: 4
---

:::note[Browser features]
Constraint validation, `:user-invalid` and `<datalist>` come from the browser.
The `check` rules and the re-rendered form are the framework's. Most of this
page is about keeping the two from disagreeing.
:::

A form is where a server-rendered application is most obviously fine and most
obviously improvable. It works: the browser submits, the handler answers, the
page comes back with errors on it. What it lacks is the half-second in which
the reader learns that the field is wrong before paying for a round trip.

The browser can supply that half-second. What it cannot do is decide what is
valid, and confusing the two is the mistake this page is about.

## Two validators, one truth

The server's `check` rules are the truth. They run on every request, including
the ones that never touched your form:

```go
type createInput struct {
	Title    string `payload:"title" check:"required,maxlen=60"`
	Owner    string `payload:"owner" check:"required,maxlen=24"`
	Priority string `payload:"priority" enum:"low,normal,high" default:"normal"`
}
```

The HTML attributes are an echo of those rules, placed where the reader is:

```html
<input id="title" name="title" value={form.title}
       required maxlength="60" autocomplete="off">
<input id="owner" name="owner" value={form.owner}
       required maxlength="24" autocomplete="off">
```

They are duplication, and worth it — but only in one direction. Attributes may
restate a server rule; they must never be the only place a rule exists. A
narrower attribute than the server's check is a bug the server will accept
silently; a wider one is just a round trip you did not save.

`required`, `maxlength`, `min`/`max`, `step`, `pattern` and the input types
(`email`, `url`, `number`, `date`) cover most of what `check` expresses.

### Style the failure, not the emptiness

`:invalid` matches an empty required field before anyone has typed in it, which
is why forms styled with it look broken on arrival. `:user-invalid` waits until
the reader has actually interacted:

```html
<head>
<style>
.field input:user-invalid { border-color: crimson }
.field input:user-invalid + .hint { display: block }
.hint { display: none }
</style>
</head>
```

Remember that a scoped selector needs a class to hang off — a bare
`input:user-invalid` fails generation. See
[Browser controls](/guides/interactivity/browser-controls/) for the rule.

## Errors that come back from the server

Client-side checks stop the obvious cases. Everything else — uniqueness, a
value that was valid until someone else changed something — is only knowable
after the request.

The classic form of the answer is to re-render the page with the errors and the
reader's own text still in the fields. `pw.Parse` returns the zero value when a
check fails, so that text comes from the request rather than from the parsed
struct:

```go
input, err := pw.Parse[createInput](r)
if err != nil {
	mapped, ok := httpbind.AsHTTPError(err)
	if !ok || len(mapped.Fields) == 0 {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	form := FormState{Title: r.PostFormValue("title"), Owner: r.PostFormValue("owner")}
	applyFieldErrors(&form, mapped.Fields)
	pw.WriteHTML(w, r, NewTask(NewTaskParams{Form: form}))
	return
}
```

The distinction in that first branch matters: field-level failures belong next
to an input, and an unreadable or oversized body does not — that one is a
problem response.

`examples/htmx_fragment` runs the same logic on the fragment path, where one
extra rule applies: a swap library ignores a non-2xx response, so a rejected
form is answered with **HTML and a 200** rather than with a problem document.
The status is not a lie about validity; it is a statement that this response is
the thing to display.

## Forms inside dialogs

A `<dialog>` can hold either kind of form, and the difference is one attribute:

```html
<dialog id="rename" class="sheet">
  <form method="dialog"><button value="cancel">Cancel</button></form>
  <form method="post" action="/tasks/rename">
    <input type="hidden" name="id" value={id}>
    <label for="title">New title</label>
    <input id="title" name="title" required maxlength="60">
    <button type="submit">Rename</button>
  </form>
</dialog>
```

`method="dialog"` closes without submitting and sets `returnValue` to the
button's `value`. The POST leaves the page entirely, and Post/Redirect/Get
brings back a fresh document in which the dialog is closed because it was never
opened. Nothing needs to remember to close it.

Constraint validation works normally inside a dialog: the browser refuses to
submit and focuses the offending field, in the top layer, with no arrangement
on your part.

The case that needs thought is a rejected submission you want to show *in* the
still-open dialog. That is a fragment swap, not a navigation — see
[Fragments and islands](/guides/interactivity/fragments/).

## Suggestions without a script

```html
<label for="owner">Owner</label>
<input id="owner" name="owner" list="owners" autocomplete="off">
<datalist id="owners">
{for owner in owners}
  <option value={owner}></option>
{/for}
</datalist>
```

`<datalist>` is a suggestion list, not a constraint — the reader may type
something not in it, which is often the point. It suits a set the server can
send with the page: team members, tags, recent values. It does not suit ten
thousand rows, or a list that has to filter server-side; that is a fragment
swap, and the same `input` gains `hx-get` and a target instead of a `list`.

## One route or two

A form that works without JavaScript and gets better with it is the point of
this ladder, and it raises a question the framework deliberately does not answer
for you: should the swap-driven submission go to the same route as the ordinary
one?

Nothing in a fragment response is derived from the client — no request is
classified, and no header is inspected on your behalf. So if one route is to
serve both, you inspect the header yourself:

```go
if r.Header.Get("HX-Request") == "true" {
	pw.WriteHTMLFragment(w, r, TaskPanel(params))
	return
}
pw.WriteHTML(w, r, Page(pageParams))
```

Two routes are usually clearer, and they let the page path keep a redirect while
the fragment path answers with markup. One route is worth it when the logic
before the response is long enough that duplicating it would be worse than the
branch.
