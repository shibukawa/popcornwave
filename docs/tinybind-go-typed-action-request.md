# Change request: a typed server function, reached by a call

Written against **tinybind-go v0.5.8** on 2026-08-13, after adopting the four
asks of `docs/tinybind-go-actions-and-handlers-request.md` end to end. Those
shipped as asked and the client half is built on them; this is one ask, and it
is the typed argument binding the server-action rung has been carrying as a
planned half since it was written.

The short version: an action's signature is fixed at `http.HandlerFunc` for a
good reason, and that reason turns out to be about the **response** rather than
the input. Narrow which callers can reach a function and the reason lapses,
which is what makes a typed shape possible without disturbing the one that
ships.

---

## What an author would write

```go
var _ = pw.ServerAction(GetUser)

func GetUser(ctx context.Context, id string) (User, error) { … }
```

and call from a component script:

```js
const user = await actions.getUser({ id: "3" });
```

No template can name it. A form reaching it would be shown a value it cannot
render, so that is a generation error rather than a runtime shape.

---

## The two things that were missing

Neither is machinery. The rung was blocked on two facts that nothing in the
source could carry.

**An arbitrary signature says nothing about being an action.** The current
admission rule *is* the signature: `isHandlerSignature` takes an exported
function with exactly the transport types and no results, which is unambiguous
precisely because nothing else has that shape. A rule over arbitrary signatures
cannot exist, because every function has one. So something outside the signature
has to say which functions are actions — and that is a declaration.

**A returned value says nothing about which response it becomes.** This is what
fixed the shape in the first place, and the reasoning is right: a form action
legitimately answers with a redirect, a conditional status, a download, or a
stream, and no fixed return covers those. But that is a fact about a caller with
a document waiting for a page. A caller holding the answer has no use for a
redirect and nowhere to apply regions — so with one caller the response is not a
choice.

---

## Why the declaration you already rejected is not this one

`requirement:template-server-functions` records:

> **rejected_declaration:** a compile-time assertion such as
> `var _ tinybind.ServerAction = Rename` … **why_not:** it costs a declaration
> for every action to restate what the package boundary already says

That was right, and it does not carry here. With the shape fixed, the
declaration bought only intentional exposure, and exposure is what the route
package boundary already bounds — so it restated something. With an arbitrary
signature it carries the one fact the boundary cannot: that this function, of a
shape nothing else distinguishes, is an action.

The rejection was of a declaration with nothing to say. This one has something
to say.

---

## The responsibility line

**Yours.** Admitting the second shape, reading the declaration, emitting the
glue, and every diagnostic that needs a source position.

**Ours.** The declaration's spelling, the namespace a script calls through, and
the runtime — which needs *nothing new*. It already posts a JSON body to an
action address and returns a non-update response to its caller, so a typed
action is exactly that pair with the Go side generated. We verified that while
building it: the client is done.

**The line this moves.** Every earlier decision in this feature kept the
generator out of the response body, deliberately. This asks it to write one.
`routetree` already does that for a page entry point — it decodes the route,
calls `Load`, and renders — so the line moves rather than breaks. We would
rather say that plainly than have it discovered in review.

---

## 1. Admit a declared function of any signature

A second admission rule beside the shape filter, not a replacement: an exported
handler-shaped function stays an action by existing, and a declared one is an
action by being declared.

**Input:** a package-level declaration naming the function. The spelling is
ours; what you need is to recognize the call and resolve its argument to a
function in the package.

**Output:** an entry in the route table beside the raw ones, and a registration
at the same `/_action/<hash>/<Name>` address, where the hash is the same digest
of the declaring directory and the Go name.

**Diagnostics:** a declaration whose argument is not a function in this package;
a typed and a raw action colliding on one name, which is the hash collision you
already refuse.

The declaration takes the symbol rather than a name string, so a declaration
naming something that does not exist fails to compile before generation reads
it.

---

## 2. Read the signature, which you already know how to do

**Parameters** bind by name from the call's JSON payload, which is the rule you
already apply to a page entry point against the URL. There is no second source:
the direct entry point holds no path parameter, so every argument comes from the
caller and no precedence rule is needed.

**A leading `context.Context` is accepted and optional**, on exactly the terms
`takesLeadingContext` already implements for a typed `Load` — recognized
syntactically, trimmed from the input list, and the request's context passed
first. We verified that half shipped in v0.5.8 by declaring one in our page tree
fixture and reading the emitted call. A typed action reuses it rather than
asking for anything.

**Results** are one value and an error, or an error alone.

**No type checking is needed.** You read a page entry point's parameters and
results from the AST because generation runs before the package compiles, and
that is enough here too: the glue constructs an argument struct and encodes a
result, and neither step reads a type's fields.

---

## 3. Emit the glue

```go
mux.HandleFunc("POST /_action/<hash>/GetUser",
    func(w http.ResponseWriter, r *http.Request) {
        input, err := decode(r)
        if err != nil { pw.WriteProblem(w, r, err); return }
        out, err := users.GetUser(r.Context(), input.ID)
        if err != nil { pw.WriteProblem(w, r, err); return }
        pw.WriteAPI(w, r, out)
    })
```

**Input:** the analysis from ask 2. **Output:** the handler above, through the
symbols a framework already repoints — the error writer and the response writer
are both settings you take today.

**Diagnostics:** a parameter or result type the codec cannot carry, named with
its position.

---

## 4. Report the published name

A script writes `actions.getUser`, and Go's export rule leaves no choice about
the identifier. So the name a caller writes is not the Go name, and it has to
reach us.

**Default:** the Go name in lowerCamelCase — the leading run of capitals
lowercased, leaving the last of the run intact when a lowercase letter follows,
so `GetUser` reads as `getUser`, `GetURL` as `getURL`, `URLFor` as `urlFor`, and
`ID` as `id`.

**Override:** an optional string on the declaration, for a name the derivation
reads wrong and for a published name a Go rename must not move. This is a wire
name rather than a second spelling of the identity — the same distinction a
struct tag already draws.

**Output:** the published name on the action's route table entry, beside the
handler name and the hash. We derive our own registration from that table
already, so a field is all we need.

---

## 5. Refuse a typed action from a template

`server-action="GetUser"` naming a typed action is a generation error, with the
position and the function.

This is what makes ask 2's answer to the response question hold: a form cannot
reach one, so the case where a native submit is shown a JSON document does not
arise rather than being handled. We currently close that case with a header the
handler branches on, and that stays for the raw shape, where both callers are
legitimate.

---

## What we are not asking for

**Replacing the raw shape.** It is load-bearing and it is what a form reaches.
A page may have both; they are different functions with different signatures.

**A result union.** Enumerating value, redirect and regions in one return type
covers every caller, and makes every author write the union where only one
member is possible, with a framework type in every signature. Narrowing the
caller gets the same answer with nothing in the signature. It stays the shape to
return to if a typed action ever needs a redirect.

**Regions from a typed action.** A handler that answers with the update regions
belongs to the raw shape. Admitting them here would put the response question
back where this design just answered it.

**OpenAPI.** A typed action is still one page's implementation detail, so the
exclusion `rule:generated-source-not-discovered` already applies is unchanged.

**A context anywhere but the first parameter.** The ordinary not-an-input
diagnostic is the right answer there.

---

## Sequencing

Asks 1 to 3 are one feature and land together; there is no useful half.

Ask 4 is separable — we can derive the published name ourselves from the Go name
and lose only the override, so if the table field is awkward, say so and we will
take the derivation and come back for the string later.

Ask 5 is a check and can follow, at the cost of an author being able to write a
form that shows a JSON document until it lands.

---

## Related catalog entries

Ours, if you want the reasoning rather than the request:
`requirement:typed-server-action`, `decision:typed-action-declaration`,
`decision:typed-action-is-call-only`, `api:server-action`,
`rule:server-action-authoring`, and `requirement:action-invocation-runtime` for
the client half that is already built.
