# Change request: WriteStatus generates no encoder, and the formatter splits trailing comments

**From:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** `github.com/shibukawa/tinybind-go` v0.4.3, both confirmed still present in v0.4.7
**Date:** 2026-08-09
**Status:** fixed in v0.4.9, both asks taken as proposed; this framework is on it

## Outcome

Ask 1 landed as the one-line usage change: `OperationResponseWriteStatus`
returns `UsageWrite | UsageEncodeJSON`, so a write-status call site now emits
`jsonbind.RegisterEncode` beside its writer. `pw.WriteStatus` calls
`httpbind.WriteStatus` directly and the wrapper that stood in for it is gone.

Ask 2 landed as the same condition `flushBefore` already used, so a comment
block that follows the last declaration keeps its spacing and formatting is a
fixed point again.

One thing the fix does not change, worth stating because this framework had to
handle it: `WriteStatus` writes its status and content type before it encodes,
so a missing encoder is discovered after the response committed. There is no
status left to answer with, and `pw.WriteStatus` logs the build mistake rather
than writing a problem document over a 2xx. Encoding into a buffer first would
close that, and it is a smaller thing than either ask above.

The rest of this document is the request as it was raised.

## Summary

Two unrelated defects, found while wiring `httpbind.WriteStatus` into Popcorn
Wave and while making our project scaffold survive `tb fmt --check`.

1. **`httpbind.WriteStatus` fails at runtime in exactly the projects generated
   for it.** It serializes through the jsonbind encoder registry, but a
   `write_status` call site registers only the httpbind *writer* — never the
   encoder — so the first call returns `missing_codec`. The feature cannot be
   used through its own entry point.
2. **The template formatter inserts a blank line between every trailing
   comment.** `flushRemaining` ignores `Comment.BlankBefore`, which
   `flushBefore` honours, so a comment block that is not followed by a
   declaration is re-spaced on every run. This contradicts
   `requirement:template-comment-retention`.

The first is a correctness bug with no workaround inside the module. The second
is cosmetic but hits every file whose comments come last — which includes the
commented-out example that our `pw init` writes into `queries/users.pw.sql`.

## Ask 1 — register the JSON encoder for a `write_status` call site

### What happens

`WriteStatus` encodes through jsonbind:

```go
// write.go:24
func WriteStatus[T any](w http.ResponseWriter, r *http.Request, status int, value T) error {
	_ = r
	if status == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return jsonbind.EncodeJSON(w, value)   // <-- lookupEncoder[T]
}
```

but `usageForCallOperation` gives a write-status call the same usage as a plain
write:

```go
// generator/options.go:481
case OperationResponseWrite, OperationResponseWriteStatus:
	return UsageWrite
```

and `UsageWrite` emits `httpbind.RegisterWrite` and nothing else:

```go
// generator/emit.go:55
if t.DirectUsage&UsageWrite != 0 {
	fmt.Fprintf(&registrations, "\thttpbind.RegisterWrite[%s](write%s)\n", t.Name, t.Name)
}
```

`emitEncode` does run for a `UsageWrite` type, so `encode<T>` exists in the
generated file — it is simply never registered. The encoder the runtime looks
up is the one thing missing.

### Reproduction

A response type used only through `WriteStatus`, generated with a call pattern
registered as documented:

```go
ResponseWriteStatusCall(
	Function(path, "WriteStatus"),
	GenericType("response", 0),
	Argument("status", 2),
)
```

The generated `init` is:

```go
func init() {
	httpbind.RegisterBind[createRequest](bindcreateRequest)
	httpbind.RegisterWrite[created](writecreated)
	// no jsonbind.RegisterEncode[created]
}
```

so `httpbind.WriteStatus[created](w, r, http.StatusCreated, v)` returns

```
jsonbind: no JSON encoder registered (missing generated init?)
```

The OpenAPI half of the feature works correctly: the document carries a `201`
referencing the `created` schema and no `200`. Only the runtime half is broken,
which is why this survived — `generator/writestatus_openapi_test.go` asserts
the document and never calls the function.

### Suggested fix

One line, mirroring what `OperationStreamCreate` already does for the same
reason (its runtime also encodes through jsonbind):

```go
// generator/options.go
case OperationResponseWrite:
	return UsageWrite
case OperationResponseWriteStatus:
	return UsageWrite | UsageEncodeJSON
```

An alternative that needs no generator change is to make `WriteStatus`
serialize through the registered writer instead of the encoder — but the writer
hard-codes its status (`WriteJSONBytes(w, http.StatusOK, ...)`), so that path
needs a status parameter to reach the writer, which is a larger change.

**A test that would have caught it:** the existing
`writestatus_openapi_test.go` builds a module and asserts the document. The
same fixture, compiled and its handler invoked through `httptest`, fails today.

### Note on the second registration workaround

Registering the same function twice, once as `ResponseWriteStatusCall` and once
as `JSONEncodeCall`, is the obvious downstream workaround and it is rejected:

```
generator: conflicting call patterns for <pkg>.WriteStatus
```

That refusal is reasonable. It just means a consumer cannot fix this from
outside the module.

### What Popcorn Wave does meanwhile

Our `pw.WriteStatus` does not call `httpbind.WriteStatus`. It calls
`httpbind.Write` through a `http.ResponseWriter` wrapper that rewrites the
writer's `200` into the requested status, and short-circuits `204` before any
serialization. That keeps us on the registered writer, which generation does
emit. We would rather delete that wrapper once Ask 1 lands.

## Ask 2 — `flushRemaining` should honour `BlankBefore`

### What happens

`flushBefore` respects the blank line the source had:

```go
// templates/internal/syntax/print.go:304
func (m *modulePrinter) flushBefore(pos Position) {
	before, rest := CommentsBefore(m.comments, pos)
	m.comments = rest
	for _, c := range before {
		if m.wrote && (c.BlankBefore || !m.afterComment) {
			m.p.Blank()
		}
		...
```

`flushRemaining` — the path for comments with no declaration after them —
inserts a blank line unconditionally:

```go
// templates/internal/syntax/print.go:317
func (m *modulePrinter) flushRemaining() {
	for _, c := range m.comments {
		if m.wrote {
			m.p.Blank()      // <-- BlankBefore and afterComment both ignored
		}
		m.writeComment(c)
		m.wrote = true
	}
	m.comments = nil
}
```

Each `//` line is its own `Comment`, so an adjacent block is exploded.

### Reproduction

Input:

```
package queries

export statement F(id: int): sql.one<E> {
  SELECT id FROM e WHERE id = {id}
}

// tail one
// tail two
```

Output:

```
package queries

export statement F(id: int): sql.one<E> {
  SELECT id
  FROM e
  WHERE id = {id}
}

// tail one

// tail two
```

The same two lines placed *above* the statement are preserved exactly, which
isolates the defect to `flushRemaining`.

A whole-file case, which is what we actually hit: a `.pw.sql` whose only
content is a package clause and a commented-out example — every declaration is
inside the comment — has all twelve of its lines separated.

### Why it matters downstream

`pw init` scaffolds `queries/users.pw.sql` as a package clause plus a
commented-out worked example, so the first `pw fmt` in a new project inflates
the reader's first look at the query language into a double-spaced block. We
now run the formatter over our scaffold literals at write time so a new project
passes `--check` immediately, which means this spacing is what every new
Popcorn Wave project ships with until it is fixed.

### Suggested fix

Make the two paths agree:

```go
func (m *modulePrinter) flushRemaining() {
	for _, c := range m.comments {
		if m.wrote && (c.BlankBefore || !m.afterComment) {
			m.p.Blank()
		}
		m.writeComment(c)
		m.wrote = true
		m.afterComment = true
	}
	m.comments = nil
}
```

`declcomment.go:112` holds a third copy of the same condition and already gets
it right, so this brings the last of three into line rather than inventing a
rule.

**Acceptance:** formatting a source whose comments follow the last declaration
returns them byte-identical, and formatting twice is a fixed point — the
property `rule:template-format-fidelity` already claims.

## Impact summary

| Ask | Severity | Blocks us? |
| --- | --- | --- |
| 1 — write-status encoder | correctness; the documented entry point cannot work | no, we wrap `Write` instead |
| 2 — trailing comment spacing | cosmetic, but every formatted file with trailing comments | no, we ship the spacing |

Neither is urgent for us. Ask 1 is worth fixing before anyone else adopts
`WriteStatus`, because the failure is a runtime error in generated code and the
cause is not visible from the call site.
