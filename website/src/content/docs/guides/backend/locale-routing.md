---
title: Locale Routing
description: Which URL decides the language, what each mode varies on, and calling messages from Go without a request.
sidebar:
  order: 7
---

Once a project has [translated pages](/guides/frontend/i18n/), something has to
decide which language a request is being served. That decision belongs to the
route rather than to the handler, because it also decides what the response can
be cached as — and a page that gets that wrong serves one reader's language to
the next.

A project serving one language needs none of this: with no `[i18n]` block there
are no modes, no negotiation, and no `Vary`. This starts mattering the moment a
second language exists.

Three modes, declared per path prefix:

```toml
[i18n]
locales = ["ja", "en"]
default_locale = "ja"
path_routes = ["/"]
cookie_routes = ["/admin/"]
header_routes = ["/api/"]
```

The longest matching prefix wins, so `/api/` above is negotiated by header even
though `/` covers it.

## Which mode a route wants

**Public pages take `path_routes`.** The language is a path segment, so two
languages are two URLs. Nothing varies, a shared cache works normally, and
`hreflang` has something to point at. This is the only mode a search engine can
index per language.

**An authenticated application takes `cookie_routes`.** The reader picked a
language in a settings screen and it should survive, so a stored choice outranks
the browser's header. The cost is real: the response varies on `Cookie`, and
HTTP cannot vary on one cookie, so a shared cache splits per session. That is
what an authenticated page already is, which is why the cost lands here and not
on the public tree.

**An API takes `header_routes`.** A native client sends what its device reports
and manages no cookie, and an API whose body changed with cookie state would be
answering the same request two ways for a reason the client never stated.

A path outside every declared prefix serves the default language with nothing in
its URL, so adding i18n to one subtree leaves the rest alone.

## What each mode varies on

| Mode | Decided by | `Vary` |
| --- | --- | --- |
| `path` | the URL prefix | nothing |
| `cookie` | the locale cookie, then `Accept-Language` | `Cookie`, `Accept-Language` |
| `header` | `Accept-Language` | `Accept-Language` |

A negotiated route varies on **every** response, not only the ones that carried
the signal. This differs from how [user preferences](/guides/frontend/responses/)
work, and the reason is worth stating: a colour scheme has a floor, because CSS
answers it correctly with no server involvement, so a response built without the
signal is right for everyone. Language has no floor. A reader arriving with no
cookie would otherwise fill a shared cache with an unvaried default that the
next reader — whose cookie says otherwise — is then handed.

Every response also carries `Content-Language`.

## The root of a path-routed site

With `path_routes`, `/about` names no language. It is negotiated and redirected
to `/ja/about`, with a `302` rather than a `301`: the target depends on who
asked, and a permanent status would cache one reader's negotiation for everyone
behind the same proxy.

The prefix position is always read as a language. `/de/about` where German is
not declared is an ordinary path, not a broken locale — which is what keeps a
route named `/de/` from breaking the day German is added.

### Dropping the prefix from the default language

```toml
prefix_default = false
```

Now `/about` is Japanese and `/ja/about` redirects to it permanently. The root
needs no negotiation redirect at all, which is one round trip saved on the visit
that matters most.

The default is `true`, and the reason is what happens later: under `false`,
changing which language is default moves every URL on the site. Under `true` it
changes only where the root redirects. Take `false` when you are confident the
primary language will not change.

## Calling a message from Go

A generated message takes the locale as its first argument, so the same function
serves a handler, a batch job, a mail renderer, and a push notification. There
is no separate library mode — the generated surface already is one.

```go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornweb/pw"
	"example.com/app/messages"
)

func Welcome(w http.ResponseWriter, r *http.Request) {
	greeting := messages.ShopGreeting(pw.LocaleContext(r.Context()), "Ada")
	pw.WriteHTML(w, r, Page(greeting))
}
```

Outside a request there is no context to read, so the locale is parsed from
whatever recorded it:

```go
func SendReceipt(ctx context.Context, account Account) error {
	locale, ok := pw.ParseLocale(account.Language)
	if !ok {
		locale = pw.DefaultLocale()
	}
	return mail.Send(account.Address, messages.MailReceipt(locale, account.Name))
}
```

`ParseLocale` reports absence rather than substituting the default, because a
stored preference that is no longer a declared language is worth knowing about.
Matching follows RFC 4647, so `ja-JP` finds `ja`.

For a URL built outside a template — a redirect target, a link in an email, a
push deep link — `pw.LocalePath` applies the same prefix rule the template
binding does, so no caller branches on the mode.

## Should the server translate an API response?

Usually not. [Problem responses](/guides/frontend/responses/) already carry a
machine-readable code, and a native client that translates it itself knows its
device language exactly, works offline, and ships text matched to its own
version. A server-chosen string is worse on all three.

Translate on the server where the client cannot: mail, push notifications,
generated documents, operator-configured text, and any client with no catalog of
its own. The framework never does it on its own initiative — you pass a rendered
string into the problem constructor when you want one.

## Pitfalls

**A locale must be one you declared, never one echoed from the request.**
`ParseLocale` enforces this. An arbitrary tag reaching a URL turns every
unmatched `Accept-Language` into a distinct address at the origin, which is an
unbounded cache surface rather than a rendering fault.

**Cookie mode is not a cheaper path mode.** It gives up shared caching entirely.
If the pages are public, the cost is the wrong way round.

**A cached component keys on the language it read.** That is automatic, and it
means a component rendering a message is not shared across languages. A
component that renders none keys exactly as it did before.

The message catalog format, plurals, and the switcher are in
[Translated pages](/guides/frontend/i18n/).
