# Change requests: invocable server actions, and a template that names a handler

Written against **tinybind-go v0.5.7** on 2026-08-11, while designing two
features together in Popcorn Wave: making `server-action` actually perform its
mutation, and letting a template name a function the component's script block
produced.

Four asks, plus one we would like to discuss rather than request. None of them
requires the module to read JavaScript — we do that on our side and hand you the
answer, using the pass structure `server-action` resolution already has.

---

## The shape of both features

A page carries two kinds of behaviour, and they are different things that happen
to sit on the same elements.

A **server action** is a destination. `server-action="Rename"` names an exported
Go function, generation resolves it to an address, and activating the element
performs a mutation. There is one per element and its trigger follows from the
element: a form submits, a button is clicked.

A **client handler** is a listener. `on-click="increment"` names a function the
component's script block produced, and our runtime binds it when the component
mounts. An element may carry several, and most of the useful events are not the
activation event.

The division of labour we want is the one `server-action` already has: you
reserve the attribute, resolve what can be resolved at generation, and lower it
to markup; we own everything that happens in a browser.

---

## The responsibility line

**Yours.** The template grammar, attribute reservation, what markup a lowering
emits, and every diagnostic that needs a source position.

**Ours.** Reading the script block, the browser runtime, the `setup` signature
and everything reachable from it, the endpoint prefix, CSRF, and the page-tree
wiring.

The one line worth stating outright, because it is what makes these asks small:
**we are not asking you to read JavaScript.** Your position that the module
reads none, and that `setup`, its argument, and the teardown convention are the
caller's, both stand unchanged. Where a decision needs to know what is inside a
block, we parse it and pass you the result as a compile option.

---

## 1. A form carrying `server-action` is a GET form today

This is a correctness fix on shipped markup, and it is the one we would take
first.

The scripted lowering set writes the URL attribute and, deliberately, no
`action`, no `method`, no selector and no token, on the grounds that a runtime
intercepts the submit. A form declaring no method is a GET form to the browser
and to your own CSRF insertion, which only writes the hidden field into an unsafe
form. So the emitted markup for

```html
<form server-action="Save"> … </form>
```

is a form the browser will submit as a **GET to the current URL with the fields
as a query string**. The handler never runs. With scripting off that happens
natively; with our runtime loaded our own GET-form interception takes it first
and does the same thing, so neither path reaches the handler.

An application cannot work around it. An author-written `action` on a form
carrying `server-action` is a generation error on your side, and the hash is not
a value an author can compute.

To be straight about how we know: this is derived from the lowering rule rather
than observed. Our fixture carries a bare button, so we have no form under the
scripted set to point at. If you read the rule differently we would rather hear
that than have you build something.

### What we are asking for

Emit the script-free form lowering — the page-pattern `action`, `method="post"`,
the hidden selector, and the token your CSRF insertion then contributes —
**alongside** the runtime attribute, from one compile, rather than as the
alternative half of the document-level mode.

We understand why the mode is exclusive on your side: the selector, the
page-pattern POST registration and the render-time path channel are costs the
scripted set does not otherwise pay, and a per-request switch would be cloaking.
Neither argument applies to emitting both, because the bytes are the same for
every client and the runtime's presence is what selects which of two mechanisms
drives the same markup. That is how every other fallback in our runtime already
works: a link works because it is a link, and the runtime makes it faster.

We cannot take the mode as offered because our acceptance criteria require one
build that works without a browser runtime and one that uses it. A per-build
mode makes those two deployments of one application.

**Input:** nothing new from us. **Output:** both attribute sets on a form.
**Cost we accept:** every project pays the selector and the page-pattern
registration whether or not any client submits without a runtime.

If you would rather expose this through the lowering profile than change the
default, that works for us too — we need the markup, not a particular seam.

---

## 2. Reserve an `on-<event>` attribute and lower it away

We want a template to name a client handler:

```html
<button on-click="increment" data-id={row.ID}>+1</button>
```

Your event attribute context rule already excludes a hyphenated `on-` name from
its handler roster, calling it a custom element's attribute rather than an event
handler content attribute. So the namespace is free, `onclick` keeps meaning
inline JavaScript unchanged, and nothing about that rule has to move.

We are not asking for `onclick="increment"` to be reinterpreted, and we do not
want it to be. A bare identifier is a valid expression statement, `RawJavaScript`
in an on-attribute is a shipped feature, and taking the reading silently is the
failure you already recorded for the script block's own marker.

### What we are asking for

Reserve an on-prefixed hyphenated attribute wherever the lowering applies, take
the resolved handler set as a compile option, and emit a lowered attribute in its
place. The authored attribute is never emitted, exactly as `server-action` is
never emitted.

**Input from us:** a map from declaration identity to the set of handler names
that declaration exposes, supplied on the same options surface the action
resolution map uses.

**Output:** one prefixed attribute per element, listing the events it binds and
the handler each maps to. We would like the value to use the grammar
`parseScopeCatalog` already reads — comma between entries, colon within one —
so that `click:increment,blur:validate` needs no second parse rule anywhere.

The reason it must be lowered rather than left in the DOM is mechanical: CSS has
no attribute-name prefix match, so finding `on-anything` means walking every
element and reading its attributes on every mount and every swap, where one
lowered marker is a single indexed `querySelectorAll`. Leaving it in the DOM
would also claim the namespace your own rule assigns to custom elements.

**Diagnostics:** an unknown name should fail generation, and the position is
yours. We would supply an explicit unresolved marker in the map rather than an
omission, so you can report with the position you hold while we supply the
reason.

**Also worth an error, and cheap:** an on-attribute on an element inside no
component declaring a script block. That needs nothing from us — no namespace
could ever resolve it.

---

## 3. Report the script block's text and the names a template referenced

This is the half that lets ask 2 and ask 4 work without you reading JavaScript.

### What we are asking for

In the pass where you already report referenced action names, additionally
report, per component declaring a script block: the block's text as authored,
and the handler names the template referenced on that component's elements.

We return a resolved map and you lower it.

We would rather not read the block ourselves. We could find it in the template
source, but duplicating your raw-text boundary rules is a drift risk we would be
creating on purpose, and the extracted asset only exists after the compile that
needs the answer.

**Input:** the compile you already run. **Output to us:** block text plus
referenced names. **Input back to you:** the resolved sets for asks 2 and 4.

---

## 4. Emit a component's parameters as data, from a set we name

A handler frequently needs a value the component was called with. Today it
cannot have one: the block is read verbatim and extracted to a content-hashed
file shared by every instance and every render, so there is nothing per-render
to interpolate — which is exactly what makes that file cacheable, and we are not
asking you to change it.

Reading the value off a rendered attribute covers most of this and is what we
will document first. What it cannot do is keep a type: a `dataset` read is text
whatever the Go value was.

### What we are asking for

Emit a JSON object onto the component's root element, holding **only** the
parameters we name, under a prefixed attribute.

**Input from us:** per component declaration, the set of parameter names to
emit. We compute it by reading which parameters the block's `setup` destructured
— the author's own code is the declaration of what crosses, so there is no
second list to keep in step.

**Output:** the attribute on the single root element, with ordinary attribute
escaping, present only where the set is non-empty.

**Type rule:** please reuse the one a reloadable component already has, refusing
a record, a slice, and `html`. It should be an error rather than a silent
omission, and for the same reason you already give — the author asked for it, by
naming the parameter in code that uses it.

**Absence:** an absent optional should omit its key, matching what the attribute
context already does when it omits the whole attribute. We do not want `null`;
it would leave JavaScript two absences to test where one will do.

**Compatibility:** a component whose block names no parameter, or that declares
no block, emits nothing and regenerates byte for byte.

---

## 5. To discuss rather than request: where a scripted action posts

Not an ask yet, because we are not sure the cost lands on your side.

The direct entry point is `/_action/<hash>/<Name>`, and it holds no path
parameter — which is what lets the lowering write it as a constant, and we are
not proposing to change that. The consequence is that a handler serving
`/users/{id}` can read the id from a script-free form submit, which posts to the
page pattern, and cannot read it from a scripted one. Two entry points to one
handler that disagree about what it can read is a contract we would rather not
document.

Options we see, cheapest first:

- **We fix it in the runtime.** It posts to the page URL and names the handler
  in a header. Nothing changes on your side, and the constant URL in the
  attribute becomes an identity rather than a target. This is probably what we
  do.
- **The lowering emits path parameters as hidden fields.** Works for a form,
  not for a bare button.
- **The attribute carries the selector rather than the URL**, making the scripted
  and script-free channels one string. Cleanest, and the only one that is
  genuinely yours.

If the first is sound from where you sit, this becomes nothing.

---

## What we are explicitly not asking for

**A JavaScript parser in the module.** We will read the block. Your config, SQL,
DynamoDB and Firestore consumers have no browser and should not carry a
tokenizer for a feature they cannot use, and the parser will be wrong about real
code at first, which is easier to fix on our release cadence than yours.

**Any opinion about `setup`.** Its signature, the bag we pass it, the teardown
convention, the signal registration, and the namespace it returns are ours, as
you already state. For context, so the asks read coherently rather than as a
request for you to support them: a handler resolves against what that instance's
`setup` returned, teardown becomes something `setup` is handed rather than
something it returns, and the returned object is a handler namespace only — no
reactivity, and nothing that asks anything of the wire.

**A trigger model.** The event in the emitted markup is one you decide from the
element kind — submit for a form, click for a button. We are not asking for an
authoring surface where a template picks an arbitrary event to fire an action
on, because that would only work with a runtime and would reopen the split ask 1
exists to close.

**Anything on the wire.** None of this adds a header, a manifest field, or a
record kind.

---

## Sequencing

Ask 1 stands alone and is a correctness fix; it needs nothing from asks 2 to 4
and closes an acceptance condition we currently fail.

Asks 2, 3 and 4 are one feature. Ask 3 is the seam the other two consume, so it
lands first among them, and asks 2 and 4 are independent of each other after
that.

---

## Related catalog entries

Ours, if you want the reasoning rather than the request:
`requirement:action-invocation-runtime`, `requirement:scriptless-action-forms`,
`decision:action-entry-point-selection`,
`requirement:component-script-event-binding`,
`decision:event-binding-attribute-spelling`,
`decision:component-handler-namespace`,
`decision:script-block-parsing-ownership`, `rule:template-behavior-attributes`,
`rule:server-action-authoring`.
