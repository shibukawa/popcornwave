# Fixed terminology

These are not style preferences. Each one names a distinction the framework
actually makes, and the wrong word tells the reader something false about how the
code works. `check_docs.mjs --only=terms` catches the phrasings that have already
appeared in drafts; the reasoning below is what you need when the wording is new.

## Routing: registered and discovered

The two routers are **registered** and **discovered**. Nothing else.

Registered routing is a handler file that calls `mux.HandleFunc` in its `init`.
Discovered routing is a directory tree under `pages/` where a directory holding a
`page.pw.html` *is* a route, and `pw generate` writes the registrations. Both
land on the same mux, and a project can carry either or both — `pw init` asks
which, and the CLI flags are literally `--router=registered` and
`--router=discovered`.

**Never** classic, modern, legacy, old-style, or new-style. Every one of those
words says the other option is on its way out, and neither is. A reader who
believes registered routing is legacy will avoid the router the tutorial teaches.

Japanese: 登録型ルーティング / 探索型ルーティング.

## `.pw.html` and `.pw.sql`: typed languages, not template engines

`.pw.html` is a **typed template language**; `.pw.sql` is a **typed query
language**. `pw generate` compiles each one into Go: a `.pw.html` component
becomes a function plus a parameter struct, so a type error or an unsafe HTML
insertion is a compile error rather than a runtime surprise.

Calling either a "template engine" (テンプレートエンジン) puts the reader back in
the world of runtime string interpolation, which is the exact thing the design
rejects. The point of the compile step is that the mistake happens at build time,
and the vocabulary has to carry that.

## `_pw_gen.go`: build output

Every `.pw.html` and `.pw.sql` compiles into a `_pw_gen.go` file **beside its
source**. Git ignores them, VS Code hides them, `pw generate` recreates them.

Say **build output** and describe editing the source. Never instruct a reader to
open, edit, patch, or fix a `_pw_gen.go` file. It is legitimate to tell a reader
to *read* one — the tutorial does, to show what generation produced — as long as
the page is clear the file is regenerated.

## `pw.ServeMux`: a type alias

On host Go, `pw.ServeMux` is a type alias for `net/http.ServeMux`. Not a wrapper,
not a facade, not a framework router. The declaration chain is
`pw.ServeMux = httpmux.ServeMux`, and under `//go:build !tinygo` that is
`type ServeMux = http.ServeMux`.

The consequence, which is what the reader needs: the patterns you register are
Go 1.22 patterns, `"GET /users/{id}"` behaves exactly as it does anywhere else,
and `r.PathValue` is the standard one. A TinyGo build (or the
`force_tinygo_logic` tag) selects a compatible implementation of the same pattern
syntax instead. Covering both targets from one import is the entire job of the
type.

Writing "wrapper" invites the reader to go looking for the framework's routing
behaviour, of which there is none.

## No difficulty labels

The site has no advanced / basic / beginner badge, in frontmatter or in prose,
and it is not gaining one. A page that needs prior knowledge says so in a
sentence at the top — see the "Prerequisite" rule in SKILL.md — because that
sentence names *which* knowledge, and a badge does not.

## Extending this list

When a new distinction earns a fixed word, add it here **and** add a pattern to
`BANNED_TERMS` in `check_docs.mjs`, then a fixture line in `tests/fixture/` so
`tests/run_tests.mjs` proves the pattern fires. A rule that lives only in prose
gets forgotten by the third contributor.
