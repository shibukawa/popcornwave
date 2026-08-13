---
title: Template Syntax
description: The complete .pw.html language — declarations, types, expressions, control flow, slots, head contributions, await boundaries, and the rules that reject a template.
sidebar:
  order: 3
---

A `.pw.html` file is a typed template language compiled to Go by `pw generate`.
Value types and HTML insertion contexts are checked at build time, so a template
that would have produced broken markup fails before the application runs.

This page is the whole language. For the shape of a page — the document shell,
the handler that renders it, when to reach for a fragment — see
[Templates](/guides/frontend/templates/).

## File layout

```html
package handlers

type User { name: string, active: bool }

enum Tone { Primary, Secondary }

external Decorate(value: string, tone: Tone): string

component Badge(label: string, children: html): html { … }

export component Card(user: User): html { … }
```

A file opens with the Go package its generated code joins. Every `.pw.html`
file in one directory compiles into one `_pw_gen.go`, so the files must agree on
the package and must not duplicate a `type`, `enum`, `external`, or `component`
name — private components included, because their generated declarations share
that package.

Generation reads only the directories `generate.templates` and `generate.pages`
list in `popcornwave.toml`, and it never descends into a child package. A
`.pw.html` outside every listed directory is reported rather than silently
skipped. See [Build Tool Configuration](/reference/build-configuration/#generate).

| Declaration | What it introduces |
| --- | --- |
| `package name` | the Go package the generated file joins |
| `type Name { field: T, … }` | a record; becomes a Go struct of the same name |
| `enum Name { A, B }` | a string enum; becomes a Go named type and one constant per member |
| `component Name(…): html { … }` | a component callable only from other templates |
| `export component Name(…): html { … }` | the same, plus a Go function |
| `external Name(…): T` | a Go function in the same package, called from markup |
| `external async Name(…): T` | the same, run concurrently and awaited |
| `external live Name(…): T` | a Go function returning a sequence the boundary re-renders on |

Record fields are separated by commas or newlines, so
`type User { name: string, active: bool }` and the multi-line form are the same
declaration.

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
| `html` | `pw.HTMLFragment` |
| `error` | `pw.AsyncError` — the value a `recover` clause binds |
| `trusted_html` | `htmlbind.TrustedHTML` |
| `trusted_css` | `htmlbind.TrustedCSS` |
| `trusted_javascript` | `htmlbind.TrustedJavaScript` |
| `script_json` | `htmlbind.ScriptJSON` |
| a declared `type` | the generated struct |
| a declared `enum` | the generated named string type |
| `T[]` | `[]T` |
| `T?` | `*T` — except `html?`, which stays a fragment because a fragment carries its own absence |
| `async T` | `pw.Pending[T]` |

`pw` re-exports what a handler holds: `HTMLFragment`, `HTMLWrapper`,
`Pending[T]`, and `AsyncError`. The four trusted types come from
`github.com/shibukawa/tinybind-go/htmlbind` directly, and a parameter usually
does not need one — the [intrinsics](#escaping-and-trusted-content) produce them
inside the template.

`async` is a prefix modifier on a parameter or a record field, and it covers the
whole type: `async Order[]` is one pending slice, not a slice of pending values.

## Generated Go shapes

```html
export component Profile(user: User, tone: Tone): html { … }
```

```go
type ProfileParams struct {
	User User
	Tone Tone
}

func Profile(params ProfileParams) pw.HTMLFragment
```

Every component takes exactly one argument — a `{Name}Params` struct with one
exported field per declared parameter, in declaration order. The rule holds for
zero, one, and many parameters, so a parameterless component still takes an
empty struct.

| Declaration | Generated |
| --- | --- |
| `export component Name(…)` | `type NameParams struct{…}` and `func Name(NameParams) pw.HTMLFragment` |
| `export component Name(children: html, …)` | additionally `func BindName(NameParams) pw.HTMLWrapper` |
| `component name(…)` | an unexported params struct and no application-facing function |
| `external Name(…)` | nothing — you write the Go function |

Only a component with an **unnamed slot** receives `Bind<Name>`, which is what
makes a leaf unusable as a wrapper and a wrapper chain a compile-time check
rather than a runtime one.

A `Fragment` is immutable and safe to share, so a parameterless wrapper can be
built once at startup.

## Expressions

An expression appears in `{…}`, in an attribute value, and in a component
argument.

| Form | Example |
| --- | --- |
| identifier | `{name}` |
| member access | `{user.profile.name}` |
| index | `{items[0]}` |
| literal | `{"text"}`, `{42}`, `{true}`, `{null}` |
| call | `{Decorate(value, tone)}` |
| unary | `{not active}`, `{!active}`, `{-count}` |
| conditional | `{active ? 'on' : 'off'}` |

| Operator | Operands | Result |
| --- | --- | --- |
| `and`, `&&`, `or`, `\|\|` | `bool`, neither optional | `bool` |
| `==`, `!=` | two assignable, comparable values | `bool` |
| `<`, `<=`, `>`, `>=` | two numbers | `bool` |
| `+` | two `string`s, or two numbers of one type | that type |
| `-`, `*`, `/`, `%` | two numbers of one type | that type |
| `not`, `!` | `bool` | `bool` |
| unary `+`, `-` | a number | that type |

There is no truthiness and no implicit conversion. `int` and `float` are
different types to arithmetic, an optional value is not a `bool`, and `null`
compares only against an optional.

## Control flow

```html
{if score >= 80}
  <strong>A</strong>
{else if score >= 60}
  <strong>B</strong>
{else}
  <strong>C</strong>
{/if}
```

```html
{for user, index in users}
  <li data-index={index}>{user.name}</li>
{/for}
```

The condition of an `if` must be `bool`. The loop index is optional; omit it
when unused. A loop iterates an array — there is no map iteration and no range
form.

`{{` … `}}` is the literal-brace escape everywhere in a template: it emits one
`{` … `}` pair and nothing inside is parsed.

## Attributes

```html
<p title={user.nickname} class="user {user.active ? 'active' : 'inactive'}">…</p>
<article hidden={not user.active}>…</article>
<a href={link.destination}>{link.label}</a>
<input disabled>
```

| Kind | Rule |
| --- | --- |
| Ordinary | takes any expression; a string is escaped for the attribute context |
| Optional | a `string?` supplying the **entire** value omits the attribute when nil |
| Mixed optional | an optional value beside static text in one attribute is a generation error |
| Boolean | emitted only when the expression is true; a bare attribute is emitted as written |
| URL | `href`, `src`, and their kin require `url`, never `string` |

The URL rule is the one that surprises most often, and it is deliberate: a
`string` in an `href` is how a `javascript:` payload reaches a page, and
`url.URL` is a value that has already been parsed.

## Components and slots

A component is called as an element named exactly as it is declared. A component
with no children may be self-closing:

```html
<Badge label={user.name}><em>member</em></Badge>
<Avatar user={user} compact={true} />
```

`children: html` receives whatever sits between the tags. `<slot>` marks where
that content lands, and the `slot` element itself is never emitted:

```html
component Panel(title: string, header: html?, children: html, footer: html?): html {
<section class="panel">
  <div class="head"><slot name="header"><b>{title}</b></slot></div>
  <div class="body"><slot required /></div>
  <slot name="footer" />
</section>
}
```

| Form | Binds |
| --- | --- |
| `<slot />` | the reserved `children` parameter |
| `<slot name="header" />` | the `header` parameter, which must be `html` or `html?` |
| `<slot>default</slot>` | the same, rendering its children when the argument is absent |
| `<slot required />` | a mandatory slot; `required` needs `html`, its absence needs `html?` |

A caller fills a named slot with a `template` element carrying that name, and
the unnamed slot with everything else:

```html
<Panel title={caption}>
  <template name="header"><em>Guide</em></template>
  <p>body text</p>
</Panel>
```

The rules that reject a slot:

- Whitespace between fill blocks is not unnamed content.
- A `template` element with no `name` is ordinary markup, emitted as written.
- A slot may sit inside an `if`, and may appear in both branches, because only
  one runs.
- A slot may **not** appear inside a `for` body, inside an `await` block, or
  twice on one render path.
- A slot argument is not a value. It cannot be read in an expression, tested for
  presence, forwarded, or inserted twice. Declare default content instead of
  testing whether the caller supplied any.
- An absent optional slot with no default leaves nothing behind — no element, no
  wrapper, no marker.

## Head contributions

A `head` element declared **outside** `<html>` is hoisted into the document head
rather than emitted where it appears:

```html
export component Card(label: string): html {
<head>
<link rel="stylesheet" href="/shared.css" />
<style>
.box { color: red }
</style>
</head>
<div class="box"><span class="label">{label}</span></div>
}
```

| Rule | Detail |
| --- | --- |
| Allowed tags | `link`, `meta`, `style`, `script`, `title`, `noscript` |
| Nesting | only `noscript` may hold elements, and only `link`, `style`, and `meta` |
| Content | static markup only; the merged head is written before the first body byte, so it cannot depend on request data |
| Bodies | `style` and `script` bodies here are raw text — no brace rule applies |
| Deduplication | identical tags are emitted once, per tag rather than per component |
| Reach | every component reachable from the rendered chain contributes, including those called from a body |

The destination is the document shell — the component that owns `html`, `head`,
and `body`. A response with no shell has nowhere to merge into, which is why
`pw.WriteHTMLFragment` treats a head contribution as an error rather than a
silent drop.

### Scoped styles

A component's `style` block is scoped by renaming the class names it declares
and rewriting the matching `class` attributes in the same component.

- Classes the block does not declare pass through unchanged, which is what lets
  Tailwind utilities coexist with scoped rules.
- `@keyframes` names are renamed too, along with the `animation` and
  `animation-name` references that reach them.
- `font-family` names and CSS custom properties stay global, so `@font-face`
  and theming still work across components.
- `:global(...)` opts one selector out.
- A bare element selector such as `p { … }` is a generation error: it carries no
  name to rename. Qualify it — `.card p { … }`.
- A class supplied through an expression cannot be rewritten and is a generation
  error.

The suffix is derived from the template path and the component name, so an
unrelated edit does not change generated class names.

### Extracted files

A `style` or `script` block with inline content never reaches the response.
Generation writes it as a file and puts a reference tag in the merged head, so
the bytes are cacheable and a Content Security Policy may forbid inline script:

```html
<link rel="stylesheet" href="/public/generated/card.style.1f0a3c9d4b21.css">
<script src="/public/generated/card.script.7c62e0b1d938.js" defer></script>
```

Style blocks of one template file bundle into one stylesheet; each component
script becomes its own file, so `defer`, `async`, `type`, and any other authored
attribute survive on its tag. The name carries a hash of the content, so an
unchanged project regenerates identical names. A `script` or `link` already
naming an external URL contributes its tag unchanged and produces no file.

## Escaping and trusted content

Strings are escaped for the position they land in — HTML text, an attribute
value, a URL. Inserting trusted content takes an explicit intrinsic:

| Intrinsic | Argument | Result type | Allowed context |
| --- | --- | --- | --- |
| `RawHTML(string)` | a string | `trusted_html` | HTML child position |
| `RawCSS(string)` | a string | `trusted_css` | inside `<style>` |
| `RawJavaScript(string)` | a string | `trusted_javascript` | inside `<script>` |
| `JsonForScript(value)` | any JSON-serialisable value | `script_json` | inside `<script>` |

`JsonForScript` refuses a value it cannot serialise, which includes anything
carrying an `async` field — a pending value has no encoding until it settles.

:::danger
`Raw*` is not a sanitiser. Never pass arbitrary external input to it. Use
`JsonForScript` rather than `RawJavaScript` whenever you are handing typed data
to the page.
:::

### Braces inside `<script>` and `<style>`

A `<script>` or `<style>` body is authored JavaScript or CSS, where `{` is
ordinary syntax. Inside those two elements a brace opens a template insertion
**only** when it is written tight against its content and takes one of these
shapes:

| Shape | Example |
| --- | --- |
| bare value | `{js}` |
| member access | `{cfg.js}` |
| call | `{JsonForScript(payload)}` |
| parenthesised expression | `{(ready ? on : off)}` |
| control block | `{if ready} … {/if}` |

Every other brace is content. The leading space is what separates the two, so
`{ name }` is content while `{name}` is an insertion — which is why an object
literal, a minified function, a nested at-rule, and a `${name}` template literal
all survive byte for byte. Anything an insertion cannot express in those shapes
is parenthesised: `{(items[0])}`, not `{items[0]}`.

Two authored forms do match a shape because they are written tight, and both are
caught rather than silently substituted:

```js
const o = {name};      // unknown identifier name
if(x){render()}        // unknown function render
```

`{{name}}` is the way out and emits `const o = {name};`. One case stays silent:
a tight shorthand whose name matches a parameter of an insertable type. That is
why the spaced form — which authored code writes far more often — is content.

A `<head>` declared outside `<html>` is a head contribution and its bodies are
verbatim, so none of this applies there.

## Whitespace

Every run of whitespace in static markup collapses to a single space at
generation time, which is what a browser renders it as. A run collapses to one
space rather than vanishing, because whitespace between two inline boxes is
visible.

These keep their bytes exactly as written:

- `<pre>` and `<textarea>`, including everything nested inside them
- `<script>` and `<style>` bodies
- any subtree marked `preserve-whitespace`

```html
<div id="log" preserve-whitespace>
  first line
  second line
</div>
```

`preserve-whitespace` is a reserved bare attribute and never reaches the output.
`preserve-whitespace="false"` is a generation error rather than a silent no-op.

Whitespace-only runs are removed outright only where the HTML parser discards
them anyway: directly inside `<html>`, `<head>`, and the table elements, and
around the doctype of a component that renders a whole document.

## External functions

```html
external Decorate(value: string, tone: Tone): string
```

```go
func Decorate(value string, tone Tone) string { … }
```

You implement the function in the same Go package with the mapped signature.
Declaring a leading `context.Context` parameter is a decision for whoever writes
the Go — the template declaration is unchanged either way, and generation reads
the package to see which functions take one:

```go
func RequestID(ctx context.Context) string { … }
```

That is how a value belonging to the request rather than to the page — a request
id, a nonce, a locale — reaches markup without travelling through the parameter
struct of every page. A function called this way must not write the response.

An external declared `: html` returns a fragment and renders as a subtree.

## Async and await

An `external async` function runs concurrently while the page renders. The Go
implementation stays an ordinary blocking function and gains an error result:

```html
external async LoadUser(id: string): User
```

```go
func LoadUser(id string) (User, error)
func LoadPosts(ctx context.Context, id string) ([]Post, error) // context optional, as above
```

An async result exists only inside the boundary that waits for it, so calling
one anywhere but an `await` binding is a generation error.

```html
{await user = LoadUser(id), posts = LoadPosts(id)}
  <h1>{user.name}</h1>
  <ul>{for post in posts}<li>{post.title}</li>{/for}</ul>
{fallback}
  <p class="pending">loading…</p>
{recover err}
  <p class="failed">{err.message}</p>
{/await}
```

| Clause | Required | Scope |
| --- | --- | --- |
| `{await a = f(), b = g()}` | yes | the bindings are visible in the primary subtree only |
| `{fallback}` | **yes** | no bindings |
| `{recover name}` | no | `name` is an `error`, with fields `code`, `message`, `retryable`, `timeout` |

The bindings after `await` start together, so two slow calls in one clause take
as long as the slower one rather than their sum. `fallback` is required because
it is what commits to the response first.

A block that omits `recover` and then fails takes the whole page's failure path
rather than rendering nothing in its place.

### Values the caller starts

An `external async` call starts when the boundary reaches it. Declare the
parameter `async` instead and the work starts wherever you start it, leaving the
template only the wait:

```html
export component Profile(customer: Customer, headline: async string?): html {
<h1>{customer.name}</h1>
{await orders = customer.orders}
  <ul>{for order in orders}<li>{order.id}</li>{/for}</ul>
{fallback}
  <p>loading {customer.name}…</p>
{/await}
}
```

```go
customer := Customer{
	Name:   "ada",
	Orders: pw.Go(ctx, func(ctx context.Context) ([]Order, error) { return store.Orders(ctx, id) }),
}
```

`pw.Go`, `pw.Resolved`, and `pw.Failed` are the three ways to produce a handle.
An `async` parameter is not callable and the only place it may be read is an
`await` binding, where it sits beside async calls in the same clause. A record
may carry settled and pending members together, which is what lets a `fallback`
render `customer.name` while the orders behind the boundary are still pending.

An unset handle of an optional type settles immediately as absent. An unset
handle of a required type is a caller bug and surfaces as
`pw.UnsetPendingError`. See
[Async rendering](/guides/cross-layer/async-rendering/).

### Live sources

```html
external live WatchMetrics(id: string): Point
```

```go
func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error]
```

A live source binds in an ordinary `await` block — there is no second clause
keyword. The leading `context.Context` is **required** here rather than
optional, because an endless source needs something that makes it return.

Every value carries the whole state of the region rather than an increment, and
the primary subtree is rendered again for each one. One clause may bind a
settle-once call and a live source together.

A live region renders output, not input. A `<form>`, `<input>`, `<textarea>`, or
`<select>` inside the primary subtree is a generation error, because a delivery
arriving while the reader is typing would discard what they typed. The rule
follows the boundary, so one live binding applies it to the whole block;
`fallback` and `recover` are exempt because no delivery replaces them. See
[Live rendering](/guides/cross-layer/live-rendering/).

## Forms and CSRF

A form whose method is `post`, `put`, `patch`, or `delete` carries a hidden CSRF
field, generated as the form's first child. You write nothing.

| Case | What happens |
| --- | --- |
| `method="get"`, or no method | No token. A GET form's fields become the query string, and a token in a URL reaches history, logs, and referrers |
| `action` is a static absolute URL | **Generation error** — inserting the token would hand your session's secret to another origin |
| `method` is an expression | **Generation error** — it cannot be classified as safe or unsafe at generation time |
| The form already has a field of that name | Left alone, so a hand-written token still works |

A component reaching an unsafe form cannot be `@cache`d, and the rule follows the
call graph. Rendering one outside a request — a mail body, a golden test — fails
rather than emitting an empty field.

## `@cache`

```html
@cache(ttl: "5m", scope: "public")
export component ProductList(rows: Product[]): html { … }
```

The annotation takes two arguments, and which of them you write decides what it
does. `ttl` asks for storage. `scope` says whose output this is. Writing neither
is a generation error, because the annotation would then ask for nothing.

### `ttl` — storing, or only declaring

With a `ttl`, the component stores its rendered bytes and reuses them until the
duration expires. It is parsed at generation time, so a malformed or
non-positive duration fails the build.

Without a `ttl`, the annotation declares scope and stores nothing. That form may
sit anywhere — an ordinary component, a layout, the document shell — because
every restriction listed below exists to protect stored bytes and this form has
none. A `ttl` on a layout or a shell is a generation error for the mirror-image
reason: the duration would describe an expiry that cannot happen.

### `scope` — who the output belongs to

`scope` takes `"private"` or `"public"`, and defaults to `"private"`.

A private component's key is prefixed with the identity of the reader it was
rendered for, so two readers never reach one entry. Popcorn Wave supplies that
value from `pw.RequestAuthentication(ctx).Subject` — the local account
identifier a session login, a passkey assertion, and a bearer token all resolve
to before any handler runs. An anonymous request has none, and a storing private
component rendered without one stores nothing rather than storing under a blank
identity.

A public component keys on its parameters alone: the component's package and
file, a fingerprint of its generated plan, and every declared parameter.

The same declaration decides what the response tells a cache, and there it has
three states rather than two.

| Declaration | Cache key | What the response reports |
| --- | --- | --- |
| none | parameters | private |
| `scope: "private"` | reader identity + parameters | private, and refuses a `public` declared around it |
| `scope: "public"` | parameters | shared, unless something else in the chain declares private |

Undeclared reports private. That is a framework default rather than a property
of the annotation: a page treated as shared that is actually per-reader serves
one reader's markup to another, while a page treated as per-reader that is
actually shared costs a cache miss. Those are not comparable, so a project that
wants the shared answer writes it once, on its document shell.

The middle row is the one worth writing deliberately. An undeclared component
inherits whatever the chain asserts — otherwise nothing could ever be public —
so `scope: "private"` is the only way to state what generation cannot see. A
component calling an external Go function that reads the reader out of `ctx`
looks shared to every check either side can write, and the annotation is what
turns the author's knowledge into a fact the call graph carries.

`@cache(scope: "public")` on a component whose call graph reaches a declared
private one is a generation error at the annotation, naming the component that
declared it. [Responses](/guides/frontend/responses/#cache-policy) covers what
reaches the wire.

### What a stored component may not do

These apply to the storing form only. Generation rejects a component whose
output could not be replayed from stored bytes:

- one declaring an `html` parameter, since a slot argument is a bound
  continuation rather than a value;
- one declaring an `async` parameter, or a record reaching an `async` field;
- one reaching an `await` boundary, directly or through a component it calls;
- one owning the document `head`, since the merged head depends on the chain;
- one reaching an unsafe `<form>`, directly or through a component it calls;
- one reaching a builtin element whose output comes from a provider, since a
  stored body would serve one request's value to whoever asks next.

The store behind it is in-process and is on by default; `html.cache.enabled`
turns it off and `html.cache.max_entries` bounds it, both in
[Configuration](/reference/configuration/#html). Private keys multiply what one
process holds by the number of active readers, so an entry cap chosen for public
keys is worth revisiting once anything is scoped. A redraw renders through the
page's own options and reaches the same store, so a component cached on the page
stays cached in the response that replaces it.

## Hyphenated elements

A hyphen is HTML's own custom-element marker, and the hyphenated element space is
a declared whitelist. Popcorn Wave declares nothing in it today, so **every
hyphenated element in a `.pw.html` is a generation error**:

```
probe.pw.html:4:6: undeclared element <my-widget> no hyphenated element is
declared, so every one is undeclared; a framework registers a builtin entry and
an application registers a passthrough entry for each Web Component it uses
```

Hyphenated names inside `<svg>` and `<math>` are standard foreign-namespace
elements and stay outside the whitelist entirely.

The `<tb-boundary>` and `<tb-apply>` elements a streamed page carries are
written by the runtime, not by a template, so they are unaffected.

## `on-<event>` inside a component script

A component declaring a [script block](/guides/interactivity/component-scripts/)
may bind one of the handlers that block returns:

```html
<button on-click="increment" on-blur="settle">+1</button>
```

Generation resolves each name against what the block's `setup` returns and
lowers every pair into one attribute:

```html
<button data-tb-on="click:increment,blur:settle">+1</button>
```

The runtime binds them when the component mounts, so the handler closes over
that instance's own state. A name the block does not return is a generation
error at the attribute that referenced it.

Four rules on the attribute itself:

- It is reserved **only inside a component declaring a script block**. Anywhere
  else an `on-`prefixed hyphenated attribute is emitted unread.
- A second hyphen is not matched, so `on-my-event` stays an ordinary
  custom-element attribute.
- The value is a name, not a call. `on-click="increment()"` is a generation
  error, because an argument list is an expression nothing could resolve.
- Two of the same event on one element is an error; the second would be lost.

`onclick` is untouched and still means inline JavaScript.

## Component parameters in the DOM

A `setup` destructuring `props` makes generation emit those parameters onto the
component's root element as JSON:

```html
<div data-tb-component="shop.card.Card" data-tb-props="{&#34;label&#34;:&#34;hi&#34;}">
```

Only the names the block destructured are emitted, which makes that
destructuring the declaration of what the component publishes to the browser.

## `server-action` in a page tree

Inside a [page tree](/guides/cross-layer/discovered-routing/), an element may
name a Go handler instead of a URL:

```html
<button server-action="Rename" data-target="#name">rename</button>
```

Generation resolves the name against the tree's handlers. What it lowers to
depends on the element, because a form can submit by itself and a button cannot.

On anything that is not a form, one attribute the runtime reads:

```html
<button data-tb-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

On a form, that attribute plus the markup a browser needs on its own — and no
`action`, so a native submit goes to the document's own URL with this page's
path parameters already in it:

```html
<form data-tb-action="/_action/d71506d06c1e/Retire" method="post">
  <input type="hidden" name="_action" value="d71506d06c1e/Retire" />
  <input type="hidden" name="_csrf" value="…" />
```

A form also makes generation register `POST` on the page's own pattern, and the
generated dispatcher branches on the hidden selector.

Every other attribute on that element survives unread, so what a click does
beyond posting there stays your markup. A name that resolves to no handler is a
generation error rather than a dead element.

See [Server actions](/guides/interactivity/server-actions/) for which element to
choose and what the handler owes.

## Common generation errors

- passing a `string` to `href`, `src`, or another URL attribute
- inserting an ordinary `string` into `<script>` or `<style>`
- mixing an optional value with static text in one attribute
- a non-`bool` condition in `if`, or a non-`bool` operand to `and`, `or`, `not`
- arithmetic on operands of two different numeric types
- an undeclared identifier, field, function, or component
- an intrinsic used in the wrong context
- a slot whose `required` marker disagrees with its parameter type, a slot the
  target component does not declare, or a slot inside `for` or `await`
- a bare element selector in a scoped style block
- calling an `external async` outside an `await` binding, or reading an `async`
  value anywhere else
- an `await` block with no `fallback`
- a form control inside a live boundary's primary subtree
- a storing `@cache` on a component declaring `html` or `async`, reaching a
  boundary, an unsafe form, a per-request builtin element, or the document head
- `@cache` carrying neither `ttl` nor `scope`, a `ttl` on a layout or shell, or
  `scope: "public"` on a component reaching a declared private one
- any hyphenated element

A diagnostic carries the template position, and one raised inside `<script>` or
`<style>` names the element and the ways out:

```
tasks.pw.html:13:65: unknown identifier name; this is inside <script> content,
where {...} is a template insertion. Write {{...}} to keep a literal brace,
insert a value with RawJavaScript or JsonForScript, or move the script to a file
under the public asset directory
```

Run `pw generate` after every template change. Until you do, the Go build still
sees the previous plan — including the diagnostic you may have already fixed.
