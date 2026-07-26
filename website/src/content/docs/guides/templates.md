---
title: Templates
description: Typed .pw.html components — parameters, control flow, slots, escaping, and scoped styles.
sidebar:
  order: 3
---

A `.pw.html` file declares typed components. `pw generate` compiles each one
into a `_pw_gen.go` file beside it. Templates are never parsed at run time:
value types and HTML insertion contexts are checked during generation.

## A component

```html
package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
```

The file opens with the Go package it belongs to. `export component` declares
the name, a typed parameter list, and the `html` result type. Generation
produces:

```go
type HomeParams struct {
	Name string
}

func Home(params HomeParams) pw.HTMLFragment
```

so the handler call is type-checked:

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

Change the parameter list and the handler stops compiling until it is updated.
A component without `export` is private and callable only from other templates.

## Types

| Template type | Go type |
| --- | --- |
| `string`, `decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime`, `date`, `time` | `time.Time` |
| `url` | `url.URL` |
| `html` | a fragment |

`T[]` is a slice and `T?` is optional. You can declare your own composites and
enumerations, which become Go structs and constants:

```html
type User {
  name: string
  active: bool
  nickname: string?
  profileURL: url
  tags: string[]
}

enum Tone { Primary, Secondary }
```

## Control flow

```html
{if active}
  <span class="active">active</span>
{else if score >= 80}
  <strong>A</strong>
{else}
  <span class="inactive">inactive</span>
{/if}
```

Conditions must be `bool` — there is no truthiness.

```html
{for user, index in users}
  <li data-index={index}>{user.name}</li>
{/for}
```

The index is optional; omit it when unused.

## Attributes

Ordinary attributes take expressions:

```html
<p class="user {user.active ? 'active' : 'inactive'}">…</p>
```

When an optional `string?` supplies the **entire** value, a nil omits the
attribute altogether. Optional values cannot be mixed with static text in the
same attribute.

Boolean attributes are emitted only when true:

```html
<article hidden={not user.active}>…</article>
```

URL attributes require the `url` type, not `string`. Passing a `string` is a
generation error — which is the point.

## Composition and slots

A `children: html` parameter receives whatever appears between the tags:

```html
component Badge(label: string, children: html): html {
<span class="badge"><strong>{label}</strong>{children}</span>
}

export component Card(user: User): html {
<Badge label={user.name}>
  <em>member</em>
</Badge>
}
```

Named slots give a component several insertion points, with defaults:

```html
component Panel(title: string, header: html?, children: html, footer: html?): html {
<section class="panel">
  <div class="head"><slot name="header"><b>{title}</b></slot></div>
  <div class="body"><slot required /></div>
  <slot name="footer" />
</section>
}
```

Callers fill them with `template` elements:

```html
export component Page(caption: string): html {
<Panel title={caption}>
  <template name="header"><em>Guide</em></template>
  <p>body text</p>
</Panel>
}
```

Three constraints are worth internalising: a slot parameter is not a value and
cannot be read in an expression, it cannot be tested for presence or forwarded,
and slots cannot appear inside a `for` loop.

The rule that keeps composition honest: **presentational components do not
fetch data.** A component renders what its parameters carry; loading happens in
the handler.

## The document shell

`templates/document.pw.html` owns `doctype`, `html`, `head`, and `body`, and
its body contains one unnamed `<slot />`:

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>My App</title></head>
<body><slot /></body></html>
}
```

Page templates provide leaf content only and never repeat the shell. The
generated document artifact registers itself during package initialisation, and
`pw generate` links that registration package into your main package without
any handler referencing it. `pw.WriteHTML` then resolves the registered
document and renders the chain with the document outermost.

So handler code never selects or constructs a document, and a missing or
duplicate registration is a **startup** error rather than a per-request
surprise.

:::caution
A project has **exactly one** `document.pw.html`. `pw generate` fails with
`multiple default documents` if it finds more than one anywhere in the tree.
Alternative shells are ordinary exported components with an unnamed slot,
selected explicitly through `pw.WriteHTMLChain`.
:::

Any exported component with an unnamed slot also gets a `Bind<Name>` wrapper
function, which is what `WriteHTMLChain` accepts:

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: id}),
)
```

Wrappers compose outermost-first, each filling the next into its unnamed slot.

## Escaping

Strings are escaped automatically, and correctly for the context they land in:

```html
<p title={message}>{message}</p>
```

Trusted content requires an explicit intrinsic:

| Intrinsic | Context |
| --- | --- |
| `RawHTML(string)` | HTML children |
| `RawCSS(string)` | inside `<style>` |
| `RawJavaScript(string)` | inside `<script>` |
| `JsonForScript(value)` | inside `<script>`, from typed data |

:::danger
`Raw*` is not a sanitiser. Never pass arbitrary external input to it. Restrict
it to fixed or previously validated trusted content.
:::

Use `JsonForScript` rather than `RawJavaScript` whenever you are handing typed
data to the page — it encodes for you.

## Component styles

A component can contribute static head content, which is how styles stay next
to the markup they belong to:

```html
export component Card(label: string): html {
<head>
<style>
.box { color: red }
</style>
</head>
<div class="box"><span>{label}</span></div>
}
```

Declared class names are renamed and the matching `class` attributes rewritten,
so styles are scoped to the component. Classes not declared in the block pass
through unchanged, which is what lets Tailwind utilities coexist with scoped
rules. `:global(...)` opts a selector out of scoping. A bare element selector is
a generation error — qualify it with a class.

## External functions

Display-specific conversions are declared in the template and implemented in Go:

```html
external Decorate(value: string, tone: Tone): string
```

```go
func Decorate(value string, tone Tone) string {
	if tone == TonePrimary {
		return "★ " + value
	}
	return value
}
```

## Multiple files in one package

Several template files in a directory combine into one generated Go file. They
must declare the same package and must not duplicate component names.

## Errors

Generation failures carry the template position:

```
profile.pw.html:12:8: html:url requires url, got string
```

The usual causes: a `string` where a `url` is required, a `string` inserted into
`<script>`, an optional value in a mixed attribute, a non-boolean condition, an
undeclared reference, an intrinsic used in the wrong context, slot markers that
disagree, and bare element selectors in scoped styles.
