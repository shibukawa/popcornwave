---
id: rule:page-directory-naming
type: rule
title: Page Directory Naming
---
A concept:page-tree directory name carries its own URL segment kind through trailing underscores, because the directory is also a Go package and the toolchain rejects an illegal import path element before it evaluates any build constraint.

```yaml
spelling:
  static: the directory name is the literal segment
  dynamic: one trailing underscore, so users/id_ serves GET /users/{id}
  catch_all: two trailing underscores, so files/rest__ serves GET /files/{rest...}
root_page: the root page registers GET /{$} rather than GET /, because a bare / is a prefix pattern in the standard library and would answer every unmatched path instead of 404
rejected_spellings:
  members: "[id], {id}, $id, @id, :id, =id, (group), -id, and ~id"
  consequence: one such directory holding Go source breaks go build ./... for the whole module, not only its own package
  handling: discovery rejects the name first and states the reason
ignored:
  members: a name starting with underscore, a name starting with a dot, and testdata
  authority: the Go toolchain already ignores them, which is what makes a private folder convention follow for free
  effect: no route tree can produce the api:page-action-endpoint prefix
absent: no spelling for a route group without a URL segment, because the bracket spelling other frameworks use is not a legal import path element
reported:
  - two directories resolving to the same route path
  - a duplicated dynamic segment name within one route
```
