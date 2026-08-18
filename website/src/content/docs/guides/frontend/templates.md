---
title: Templates
description: Typed .pw.html components — parameters, control flow, slots, escaping, and scoped styles.
sidebar:
  order: 4
---

A template normally fails only after data reaches it. A `.pw.html` component
moves that failure earlier: `pw generate` compiles it into a `_pw_gen.go` file
beside the source, checking value types and HTML insertion contexts before the
application runs.

## Code generation

A `.pw.html` file is never read at runtime. `pw generate` compiles each one into
a `_pw_gen.go` file beside it, and the application links that Go rather than the
template. The generated file is build output: Git ignores it, VS Code hides it,
and regenerating recreates it. Edit the `.pw.html`.

Three commands run it. `pw dev` watches the project's sources and regenerates
whenever one changes, then rebuilds and restarts, so a template error arrives as
a build failure moments after the save. `pw build` generates before it compiles,
and [`pw generate`](/pw/project/generate/) is that same work stopping short of
the compiler, for a build that TinyGo or your own `go build` drives — or for
running it once by hand.

The scan is not the whole module. `popcornwave.toml` names directories per
purpose, and `.pw.html` belongs to the `templates` purpose:

```toml
[generate]
templates = ["handlers", "templates"]
```

Two directories, because a page template sits beside the handler that renders
it while the document shell and the error pages live in `templates/`. Each is
walked recursively. A `.pw.html` outside all of them is reported and skipped
rather than failing the run, which is what keeps a sample or a fixture beside
your code from breaking the build:

```
pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it
```

[`pw generate`](/pw/project/generate/) lists every purpose.

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

The generated API makes the handler call type-checked:

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

Renaming a parameter renames the field, and changing its type changes the
field's type, so the handler stops compiling until it catches up. Adding a
parameter is the quiet case: the struct literal still compiles, and the new
field arrives at its zero value until a caller fills it in.
A component without `export` remains private and can be called only from other
templates.

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

The type is only half of it, because a `url` can still name a scheme the browser
executes rather than follows. `javascript:alert(1)` contains none of the
characters HTML escaping touches, so escaping it changes nothing and the result
runs. Every URL-bearing attribute is therefore checked against a scheme
allowlist — `http`, `https`, `mailto`, `tel`, plus any relative form, which
cannot leave the origin the document already has. Anything else renders as
`#tb-blocked-url`:

```html
<a href={user.website}>profile</a>
```

| `user.website` | rendered |
| --- | --- |
| `https://example.com/u` | `href="https://example.com/u"` |
| `/u/42` | `href="/u/42"` |
| `javascript:alert(1)` | `href="#tb-blocked-url"` |
| `data:text/html;base64,…` | `href="#tb-blocked-url"` |
| `data:image/png;base64,…` | `href="data:image/png;base64,…"` |

A refused URL is replaced rather than dropped, because a missing `href` looks
exactly like an attribute the template never wrote — and a URL rejected in error
would then leave nothing to find it by. The marker is a fragment, so following
it reaches the current document and nothing else.

Inline `data:` URLs survive for images, since an inline image is ordinary
authoring, but only for an exact list of media types. `image/svg+xml` is not on
it: an SVG document carries script, so it is a script sink wearing an image's
media type.

The check covers the attributes a browser resolves, not just `href` and `src` —
`xlink:href`, `data`, `cite`, `background`, `poster` and the obsolete plugin
attributes among them. `srcset` and `ping` hold several URLs each, and are
checked one entry at a time so a single bad candidate does not discard the rest.

An application that needs another scheme says so once, where it renders:

```go
pw.WriteHTML(w, r, page, htmlbind.WithURLSchemes("http", "https", "mailto", "tel", "ftp"))
```

Passing the option replaces the list rather than adding to it, so name every
scheme the page uses. `htmlbind.WithDataURLMediaTypes` does the same for the
inline-image roster.

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

Slots compose markup, but they are not ordinary values. A slot parameter cannot
be read in an expression, tested for presence, or forwarded, and a slot cannot
appear inside a `for` loop.

That restriction reinforces a broader boundary: **presentational components do
not fetch data.** A component renders what its parameters carry; the handler
loads those values.

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

Handler code therefore neither selects nor constructs a document. A missing or
duplicate registration fails at **startup**, before a request can discover it.

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

Type checking prevents one class of template error; contextual escaping handles
another. Strings are escaped automatically for the position where they land:

```html
<p title={message}>{message}</p>
```

Escaping is the answer for text and attribute values. It is not the answer for a
URL, where the danger is the scheme rather than the characters — see [URL
attributes](#attributes) above for what happens there instead.

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

## Forms and CSRF

A form that changes something needs a token proving the request came from your
page. You do not write it.

```html
<form method="post" action="/orders">
  <button>Buy</button>
</form>
```

Generation puts a hidden field carrying the token as the form's first child, so
a later field cannot displace it and no author has to remember it. A GET form
gets nothing: its fields become the query string, and a token in a URL reaches
history, logs, and referrers.

Two shapes fail generation rather than producing a form that half works:

- a form posting to another origin, which would hand your session's secret to a
  third party;
- a form whose method is a computed value, which cannot be classified as safe or
  unsafe — dropping the token would expose it on a GET, and keeping it would
  leave the form unprotected.

One constraint follows from this. A component containing an unsafe form cannot
be output-cached, because a stored body would hand one session's token to the
next visitor. The fix is to split what is cacheable from what carries the token:

```html
@cache(ttl: "1m", scope: "public") component ProductList(rows: Product[]): html { … }
component OrderForm(): html { <form method="post">…</form> }
export component Page(rows: Product[]): html { <ProductList rows={rows} /><OrderForm /> }
```

Rendering a page with an unsafe form outside a request — a mail body, a golden
test — has no session to take a token from, and the render fails rather than
emitting an empty field. That failure is the point: an empty token submits, is
rejected, and leaves nothing pointing at the cause.

See [Security](/guides/architecture/security/) for what happens to the token
after it leaves the template.

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

Declared class names are renamed, and matching `class` attributes are rewritten,
so the styles remain scoped to the component. Undeclared classes pass through
unchanged; that distinction lets Tailwind utilities coexist with scoped rules.
Use `:global(...)` to opt a selector out. A bare element selector fails
generation and must be qualified with a class.

The same declaration can keep instance-local JavaScript beside its markup in a
`<script component>` block. Its `setup` and teardown behavior, including what
happens during partial and live replacement, is covered in [Component
scripts](/guides/interactivity/component-scripts/).

## External functions

Go a template may call is declared in the template and implemented beside it:

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

A display helper like that is the small case. The same declaration is how a
component **fetches**, which is the one worth learning:

```html
external LoadUser(id: string): User

export component UserCard(id: string): html {
{val user = LoadUser(id)}
<article>
  <h2>{user.name}</h2>
  <p>{user.email}</p>
</article>
}
```

`{val …}` names the result. Without it each mention of `LoadUser(id)` is another
call, so the three fields above would be three loads — which is why a component
could not honestly fetch anything until the binding existed.

The binding has no closer. Its name is readable to the end of the enclosing
block, and its value is computed at the top of that block however much markup
comes first. That last part is what lets a loader decide the response: give the
Go function a trailing `error` and return `pw.NotFound(…)` from it, and the page
answers 404 with nothing committed.

Sometimes the call answers nothing except whether the page may render at all —
an authorization, a precondition. Declare it with no result type and write
`{check …}`, which is the same directive minus the binding:

```html
external Authorize(id: string)
external LoadUser(id: string): User

export component UserCard(id: string): html {
{check Authorize(id)}
{val user = LoadUser(id)}
<article>
  <h2>{user.name}</h2>
</article>
}
```

```go
func Authorize(ctx context.Context, id string) error {
	if pw.RequestAuthenticationContext(ctx).Subject != id {
		return pw.Forbidden("not yours")
	}
	return nil
}
```

The trailing error picks the response exactly as the loader's does, and nothing
had to invent a result type or a reader for a value the guard never had. The
leading `context.Context` is optional here as everywhere; generation reads the
Go source to see whether you took one. Keep a guard out of a **storing**
`@cache`, though: a cache hit skips everything inside the component, the check
included.

Two consequences worth having in mind from the start.

**The component's parameter is the identifier, not the record.** That is what
makes it cacheable: [`@cache`](/guides/frontend/rendering-cache/#caching-a-components-own-load)
keys on declared parameters, so one annotation here covers the load and the
render together. A component taking the loaded `User` instead could not be
cached usefully, because computing its key would need the load.

**A loading external is synchronous, so it blocks the render.** Reach for
[`await`](/guides/cross-layer/async-rendering/) instead when you want a fallback
on screen while the work runs — and note that the two are exclusive: a storing
`@cache` is refused on a component that awaits.

## Multiple files in one package

Several template files in a directory combine into one generated Go file. They
must declare the same package and must not duplicate component names.

## Errors

Generation failures carry the template position:

```
profile.pw.html:12:8: html:url requires url, got string
```

Common causes include a `string` where a `url` is required, a `string` inserted
into `<script>`, an optional value mixed with static attribute text, a
non-boolean condition, an undeclared reference, an intrinsic in the wrong
context, incompatible slot markers, or a bare element selector in scoped CSS.

The complete language — every declaration, operator, slot rule, whitespace rule,
and the full list of what generation rejects — is
[HTML Template Format](/reference/template-syntax/).
